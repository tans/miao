package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tans/miao/internal/pocketbase"
)

func (s *Server) routesFiles() {
	s.Mux.HandleFunc("GET /api/apps/{id}/files", s.auth(s.listAppFiles))
	s.Mux.HandleFunc("POST /api/apps/{id}/files", s.auth(s.createAppFile))
	s.Mux.HandleFunc("GET /api/apps/{id}/files/{fileId}/content", s.auth(s.appFileContent))
	s.Mux.HandleFunc("POST /api/apps/{id}/files/{fileId}/attach", s.auth(s.attachAppFile))
	s.Mux.HandleFunc("GET /api/apps/{id}/files/{fileId}/download", s.auth(s.downloadAppFile))
}

func (s *Server) scopedAppFile(ctx context.Context, r *http.Request) (map[string]any, map[string]any, bool) {
	app, _, err := s.appForRequest(ctx, r)
	if err != nil { return nil, nil, false }
	file, err := s.PB.Get(ctx, "app_files", pathID(r, "fileId"))
	if err != nil || file["tenant_id"] != who(r).Tenant["id"] || file["app_id"] != app["id"] { return app, nil, false }
	return app, file, true
}

func (s *Server) listAppFiles(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r); defer cancel()
	app, _, err := s.appForRequest(ctx, r); if err != nil { writeError(w, 404, "应用不存在或你没有访问权限"); return }
	id := who(r); page := queryInt(r, "page", 1, 1, 100000)
	rows, total, pages, err := s.PB.List(ctx, "app_files", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "-created", page, 25)
	if err != nil { writeError(w, 503, "文件列表暂时不可用"); return }
	items := make([]map[string]any, 0, len(rows)); for _, row := range rows { items = append(items, map[string]any{"id": row["id"], "name": row["name"], "created_at": row["created"]}) }
	writeJSON(w, 200, map[string]any{"items": items, "page": page, "perPage": 25, "totalItems": total, "totalPages": pages})
}

func (s *Server) createAppFile(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r); defer cancel()
	app, role, err := s.appForRequest(ctx, r); if err != nil { writeError(w, 404, "应用不存在或你没有访问权限"); return }
	if role == "viewer" || boolValue(app["archived"]) { writeError(w, 403, "没有文件上传权限"); return }
	input := mapBody(r); name := stringValue(input["name"]); encoded := stringValue(input["base64"])
	ext := strings.ToLower(filepath.Ext(name))
	allowed := map[string]bool{".csv":true, ".xlsx":true, ".txt":true, ".md":true, ".pdf":true, ".png":true, ".jpg":true, ".jpeg":true, ".webp":true}
	if !allowed[ext] || name == "" || len(encoded) > 6990508 { writeError(w, 400, "文件名称或内容无效，支持 CSV、XLSX、文本、PDF 和图片"); return }
	encoded = strings.TrimPrefix(encoded, "data:")
	if at := strings.Index(encoded, ";base64,"); at >= 0 { encoded = encoded[at+8:] }
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) == 0 || len(data) > 5<<20 { writeError(w, 400, "文件必须为 1 字节到 5 MB"); return }
	name = path.Base(strings.ReplaceAll(name, "\\", "/")); name = clip(strings.Map(func(ch rune) rune { if ch < 32 || strings.ContainsRune("<>:\"|?*", ch) { return '_' }; return ch }, name), 120)
	id := who(r)
	file, err := s.PB.UploadNew(ctx, "app_files", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "user_id": id.User["id"], "name": name}, []pocketbase.Upload{{Name:"file", Filename:name, ContentType:contentTypeFor(name), Data:data}})
	if err != nil { writeError(w, 503, "文件上传失败"); return }
	writeJSON(w, 201, map[string]any{"id": file["id"], "name": name})
}

func (s *Server) appFileBytes(ctx context.Context, file map[string]any) ([]byte, string, string, error) {
	return s.PB.ProtectedFile(ctx, "app_files", stringValue(file["id"]), stringValue(file["file"]))
}

func (s *Server) appFileContent(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 45*timeSecond); defer cancel()
	_, file, ok := s.scopedAppFile(ctx, r); if !ok { writeError(w, 404, "文件不存在"); return }
	name := stringValue(file["name"]); ext := strings.ToLower(filepath.Ext(name))
	if ext != ".csv" && ext != ".xlsx" && ext != ".txt" && ext != ".md" { writeError(w, 400, "此文件只支持下载，未提供 OCR 或 PDF 文本提取"); return }
	data, _, _, err := s.appFileBytes(ctx, file); if err != nil { writeError(w, 503, "文件读取失败"); return }
	result, err := readSpreadsheet(data, name, r.URL.Query().Get("sheet")); if err != nil { writeError(w, 400, err.Error()); return }
	result["id"], result["name"] = file["id"], name
	writeJSON(w, 200, result)
}

func (s *Server) attachAppFile(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 45*timeSecond); defer cancel()
	app, file, ok := s.scopedAppFile(ctx, r); if !ok { writeError(w, 404, "文件不存在"); return }
	role := s.appPermission(ctx, app, who(r)); if role == "viewer" || boolValue(app["archived"]) { writeError(w, 403, "没有记录修改权限"); return }
	input := mapBody(r); slug, recordID, fieldName := stringValue(input["table"]), stringValue(input["record_id"]), stringValue(input["field"]); expected := stringValue(input["expected_updated_at"])
	if slug == "" || recordID == "" || fieldName == "" || expected == "" { writeError(w, 400, "需要目标表、记录、附件字段和更新时间"); return }
	table, err := s.PB.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "slug = "+pbFilterString(slug)))
	if err != nil { writeError(w, 404, "数据表不存在"); return }
	field := findField(asSliceMap(table["fields"]), fieldName); if field == nil || field["type"] != "file" { writeError(w, 400, "目标字段不是附件字段"); return }
	row, err := s.PB.Get(ctx, stringValue(table["pb_collection"]), recordID); id := who(r)
	if err != nil || row["tenant_id"] != id.Tenant["id"] || row["app_id"] != app["id"] { writeError(w, 404, "记录不存在"); return }
	if stringValue(row["updated"]) != expected { writeError(w, 409, "记录已变化，请重新读取后操作"); return }
	ext := strings.ToLower(filepath.Ext(stringValue(file["name"])))
	if ext == ".csv" || ext == ".xlsx" { writeError(w, 400, "CSV/XLSX 请使用导入工具，不能作为业务附件"); return }
	if !map[string]bool{".png":true, ".jpg":true, ".jpeg":true, ".webp":true, ".pdf":true, ".txt":true, ".md":true}[ext] { writeError(w, 400, "此文件类型不能作为业务附件"); return }
	data, _, filename, err := s.appFileBytes(ctx, file); if err != nil { writeError(w, 503, "文件读取失败"); return }
	saved, err := s.PB.UploadBusiness(ctx, stringValue(table["pb_collection"]), recordID, map[string]any{}, []pocketbase.Upload{{Name:fieldName, Filename:filename, ContentType:contentTypeFor(filename), Data:data}}, expected, stringValue(id.User["id"]), "interactive")
	if err != nil { writeError(w, 409, "记录已变化或附件字段校验失败"); return }
	s.processRecordAutomation(ctx, stringValue(id.Tenant["id"]), stringValue(app["id"]), slug, "updated", row, saved)
	writeJSON(w, 200, publicRecord(saved))
}

func (s *Server) downloadAppFile(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 45*timeSecond); defer cancel()
	_, file, ok := s.scopedAppFile(ctx, r); if !ok { writeError(w, 404, "文件不存在"); return }
	data, contentType, name, err := s.appFileBytes(ctx, file); if err != nil { writeError(w, 503, "文件读取失败"); return }
	if contentType == "" { contentType = contentTypeFor(name) }
	w.Header().Set("Content-Type", contentType); w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name)); w.WriteHeader(200); _, _ = w.Write(data)
}

const timeSecond = 1000000000

func contentTypeFor(name string) string {
	switch strings.ToLower(filepath.Ext(name)) { case ".csv": return "text/csv"; case ".xlsx": return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"; case ".txt", ".md": return "text/plain"; case ".pdf": return "application/pdf"; case ".png": return "image/png"; case ".jpg", ".jpeg": return "image/jpeg"; case ".webp": return "image/webp" }
	return "application/octet-stream"
}

func normalizeSheetRows(rows [][]string) (map[string]any, error) {
	if len(rows) == 0 { return nil, fmt.Errorf("文件没有内容") }
	headers := make([]string, len(rows[0])); seen := map[string]bool{}
	if len(headers) == 0 || len(headers) > 100 { return nil, fmt.Errorf("表头必须非空且不重复，最多 100 列") }
	for i, value := range rows[0] { headers[i] = strings.TrimSpace(strings.TrimPrefix(value, "\ufeff")); if headers[i] == "" || seen[headers[i]] { return nil, fmt.Errorf("表头必须非空且不重复，最多 100 列") }; seen[headers[i]] = true }
	count := len(rows)-1; truncated := count > 100; if count > 100 { count = 100 }
	data := make([]map[string]string, 0, count)
	for _, row := range rows[1:count+1] { item := map[string]string{}; for i, header := range headers { if i < len(row) { item[header] = row[i] } else { item[header] = "" } }; data = append(data, item) }
	return map[string]any{"headers":headers, "rows":data, "truncated":truncated}, nil
}

func readSpreadsheet(data []byte, name, sheetName string) (map[string]any, error) {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".csv" {
		reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef,0xbb,0xbf}))); reader.FieldsPerRecord = 0; reader.LazyQuotes = false; reader.ReuseRecord = false
		rows := [][]string{}; for len(rows) < 102 { row, err := reader.Read(); if err == io.EOF { break }; if err != nil { return nil, fmt.Errorf("CSV 格式无效：%s", err.Error()) }; empty := true; for _, cell := range row { if strings.TrimSpace(cell) != "" { empty = false; break } }; if !empty { rows = append(rows, row) } }
		return normalizeSheetRows(rows)
	}
	if ext != ".xlsx" {
		text := string(data); cut := []rune(text); truncated := len(cut) > 30000; if truncated { cut = cut[:30000] }
		return map[string]any{"text":string(cut), "truncated":truncated}, nil
	}
	return readXLSX(data, sheetName)
}

type xlsxCell struct { Ref string `xml:"r,attr"`; Type string `xml:"t,attr"`; Value string `xml:"v"`; Inline struct { Text string `xml:"t"` } `xml:"is"` }
type xlsxRow struct { Cells []xlsxCell `xml:"c"` }

func readXLSX(data []byte, sheetName string) (map[string]any, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); if err != nil { return nil, fmt.Errorf("Excel 文件无效") }
	files := map[string]*zip.File{}; total := uint64(0)
	if len(archive.File) > 2000 { return nil, fmt.Errorf("Excel 解压后超过读取限制") }
	for _, f := range archive.File { total += f.UncompressedSize64; if total > 32<<20 { return nil, fmt.Errorf("Excel 解压后超过读取限制") }; files[path.Clean(f.Name)] = f }
	read := func(name string) ([]byte,error) { f:=files[path.Clean(name)];if f==nil{return nil,fmt.Errorf("Excel 工作簿结构无效")};r,e:=f.Open();if e!=nil{return nil,e};defer r.Close();return io.ReadAll(io.LimitReader(r,32<<20)) }
	workbook, err := read("xl/workbook.xml"); if err != nil { return nil, err }
	type sheetInfo struct { Name, RID, Target string }; sheets:=[]sheetInfo{}
	dec:=xml.NewDecoder(bytes.NewReader(workbook)); for { tok,e:=dec.Token();if e==io.EOF{break};if e!=nil{return nil,fmt.Errorf("Excel 工作簿无效")};start,ok:=tok.(xml.StartElement);if !ok||start.Name.Local!="sheet"{continue};item:=sheetInfo{};for _,a:=range start.Attr{switch a.Name.Local{case "name":item.Name=a.Value;case "id":item.RID=a.Value}};if item.Name!=""{sheets=append(sheets,item)} }
	if len(sheets)==0 { return nil, fmt.Errorf("工作簿没有工作表") }
	rels, _ := read("xl/_rels/workbook.xml.rels"); targets:=map[string]string{}; if len(rels)>0 { d:=xml.NewDecoder(bytes.NewReader(rels));for{tok,e:=d.Token();if e==io.EOF{break};if e!=nil{break};start,ok:=tok.(xml.StartElement);if !ok||start.Name.Local!="Relationship"{continue};id,target:="","";for _,a:=range start.Attr{if a.Name.Local=="Id"{id=a.Value};if a.Name.Local=="Target"{target=a.Value}};targets[id]=target} }
	var shared []string
	if raw,e:=read("xl/sharedStrings.xml");e==nil { type si struct { Text string `xml:"t"`; Runs []struct{Text string `xml:"t"`} `xml:"r"` };var table struct{ Items []si `xml:"si"` };if xml.Unmarshal(raw,&table)==nil { for _,it:=range table.Items { value:=it.Text;for _,run:=range it.Runs{value+=run.Text};shared=append(shared,value) } } }
	chosen:=0;if sheetName!="" { chosen=-1;for i,item:=range sheets{if item.Name==sheetName{chosen=i;break}};if chosen<0{return nil,fmt.Errorf("工作表不存在")} }
	allSheets:=make([]string,0,len(sheets));for _,item:=range sheets{allSheets=append(allSheets,item.Name)}
	target:=targets[sheets[chosen].RID];if target==""{target=fmt.Sprintf("worksheets/sheet%d.xml",chosen+1)};if strings.HasPrefix(target,"/"){target=strings.TrimPrefix(target,"/")}else{target=path.Join("xl",target)}
	worksheet,err:=read(target);if err!=nil{return nil,fmt.Errorf("工作表不存在")};rows:=[][]string{};d:=xml.NewDecoder(bytes.NewReader(worksheet));for{tok,e:=d.Token();if e==io.EOF{break};if e!=nil{return nil,fmt.Errorf("工作表格式无效")};start,ok:=tok.(xml.StartElement);if !ok||start.Name.Local!="row"{continue};var row xlsxRow;if e=d.DecodeElement(&row,&start);e!=nil{return nil,fmt.Errorf("工作表格式无效")};cells:=[]string{};for _,cell:=range row.Cells{column:=xlsxColumn(cell.Ref);if column<1||column>100{return nil,fmt.Errorf("工作表最多 100 列")};for len(cells)<column{cells=append(cells,"")};value:=cell.Value;switch cell.Type{case "s":index,_:=strconv.Atoi(value);if index>=0&&index<len(shared){value=shared[index]}else{value=""};case "inlineStr":value=cell.Inline.Text;case "b":if value=="1"{value="true"}else{value="false"}};cells[column-1]=value};rows=append(rows,cells);if len(rows)>=102{break} }
	result,err:=normalizeSheetRows(rows);if err!=nil{return nil,err};result["sheet"],result["sheets"],result["truncated"]=sheets[chosen].Name,allSheets,len(rows)>=102;return result,nil
}

func xlsxColumn(ref string) int { n:=0;for _,r:=range ref{if r<'A'||r>'Z'{break};n=n*26+int(r-'A'+1)};return n }
