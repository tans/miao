package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var jobLocks sync.Map
func lockJob(id string) func() { value, _ := jobLocks.LoadOrStore(id, &sync.Mutex{}); mutex := value.(*sync.Mutex); mutex.Lock(); return mutex.Unlock }

func (s *Server) routesOperations() {
	s.Mux.HandleFunc("POST /api/apps/{id}/query", s.auth(s.queryRecords))
	s.Mux.HandleFunc("POST /api/apps/{id}/batch-plans", s.auth(s.createBatchPlan))
	s.Mux.HandleFunc("GET /api/apps/{id}/batch-plans/{jobId}", s.auth(s.getBatchPlan))
	s.Mux.HandleFunc("POST /api/apps/{id}/batch-plans/{jobId}/commit", s.auth(s.commitBatchPlan))
	s.Mux.HandleFunc("POST /api/apps/{id}/import-plans", s.auth(s.createImportPlan))
	s.Mux.HandleFunc("GET /api/apps/{id}/import-plans/{planId}", s.auth(s.getImportPlan))
	s.Mux.HandleFunc("POST /api/apps/{id}/import-plans/{planId}/commit", s.auth(s.commitImportPlan))
}

func (s *Server) operationTable(ctx context.Context, r *http.Request, input map[string]any) (map[string]any, map[string]any, error) {
	app, _, err := s.appForRequest(ctx, r); if err != nil { return nil, nil, err }
	slug := defaultString(stringValue(input["table"]), r.URL.Query().Get("table"))
	if slug == "" { return app, nil, fmt.Errorf("missing table") }
	table, err := s.PB.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "slug = "+pbFilterString(slug)))
	return app, table, err
}

func buildRecordFilter(conditions any, fields []map[string]any, tenantID, appID string) (string, error) {
	list := anySlice(conditions); if len(list) > 8 { return "", fmt.Errorf("查询条件最多 8 项") }
	parts := []string{"tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(appID)}
	for _, raw := range list {
		cond := asMap(raw); name, op := stringValue(cond["field"]), stringValue(cond["op"]); field := findField(fields, name)
		if field == nil || !contains([]string{"eq", "contains", "before", "after", "empty"}, op) { return "", fmt.Errorf("查询字段或操作符无效") }
		typ := stringValue(field["type"])
		if (op == "before" || op == "after") && typ != "date" { return "", fmt.Errorf("日期条件必须使用日期字段") }
		if op == "contains" && !contains([]string{"text", "email", "url"}, typ) { return "", fmt.Errorf("包含查询只能用于文本字段") }
		if op == "empty" { parts = append(parts, name+` = ""`); continue }
		value := cond["value"]; if typ == "number" { n, ok := numeric(value); if !ok { return "", fmt.Errorf("查询值类型无效") }; value = strconv.FormatFloat(n, 'f', -1, 64) }
		if typ == "bool" { if value == true || value == "true" { value = "true" } else if value == false || value == "false" { value = "false" } else { return "", fmt.Errorf("查询值类型无效") } }
		if typ != "number" && typ != "bool" { text, ok := value.(string); if !ok { return "", fmt.Errorf("查询值类型无效") }; value = pbFilterString(text) } else { value = fmt.Sprint(value) }
		operator := map[string]string{"eq":"=", "contains":"~", "before":"<", "after":">"}[op]
		parts = append(parts, name+" "+operator+" "+fmt.Sprint(value))
	}
	return listFilter(parts...), nil
}

func numeric(value any) (float64, bool) { switch n := value.(type) { case float64: return n, n == n; case int: return float64(n), true; case string: parsed, err := strconv.ParseFloat(n, 64); return parsed, err == nil && parsed == parsed }; return 0, false }

func (s *Server) queryRecords(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r); defer cancel(); input := mapBody(r)
	app, table, err := s.operationTable(ctx, r, input); if err != nil { writeError(w, 404, "数据表不存在"); return }
	filter, err := buildRecordFilter(input["conditions"], asSliceMap(table["fields"]), stringValue(who(r).Tenant["id"]), stringValue(app["id"]))
	if err != nil { writeError(w, 400, err.Error()); return }
	page := queryIntFrom(input, "page", 1, 1, 100000)
	rows, total, pages, err := s.PB.List(ctx, stringValue(table["pb_collection"]), filter, "-created", page, 25)
	if err != nil { writeError(w, 503, "记录查询暂时不可用"); return }
	items := []map[string]any{}; for _, row := range rows { item := publicRecord(row); item["updated_at"] = row["updated"]; delete(item, "created_at"); items = append(items, item) }
	writeJSON(w, 200, map[string]any{"items": items, "totalItems": total, "page": page, "totalPages": pages, "conditions": anySlice(input["conditions"])})
}

func (s *Server) createBatchPlan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r); defer cancel(); input := mapBody(r)
	app, table, err := s.operationTable(ctx, r, input); if err != nil { writeError(w, 404, "数据表不存在"); return }
	id := who(r); if !s.appCanBatch(ctx, app, id) { writeError(w, 403, "没有批量修改权限"); return }
	change := asMap(input["change"]); field := findField(asSliceMap(table["fields"]), stringValue(change["field"]))
	if field == nil || field["type"] == "file" || field["type"] == "relation" { writeError(w, 400, "批量修改字段无效"); return }
	if msg := validateData(map[string]any{stringValue(field["name"]):change["value"]}, []map[string]any{field}, true); msg != "" { writeError(w, 400, "批量修改值无效"); return }
	filter, err := buildRecordFilter(input["conditions"], asSliceMap(table["fields"]), stringValue(id.Tenant["id"]), stringValue(app["id"])); if err != nil { writeError(w, 400, err.Error()); return }
	rows, total, _, err := s.PB.List(ctx, stringValue(table["pb_collection"]), filter, "created", 1, 101); if err != nil { writeError(w, 503, "目标记录检查失败"); return }
	if total < 1 || total > 100 { writeError(w, 400, "请把目标范围缩小到 1–100 条记录"); return }
	targets := []map[string]any{}; sample := []map[string]any{}; for i, row := range rows { targets = append(targets, map[string]any{"id":row["id"], "updated_at":row["updated"]}); if i < 10 { sample = append(sample, publicRecord(row)) } }
	job, err := s.PB.Create(ctx, "batch_jobs", map[string]any{"tenant_id":id.Tenant["id"], "app_id":app["id"], "user_id":id.User["id"], "status":"planned", "plan":map[string]any{"table":table["slug"], "conditions":anySlice(input["conditions"]), "change":change, "targets":targets}})
	if err != nil { writeError(w, 503, "批量计划创建失败"); return }
	writeJSON(w, 201, map[string]any{"plan_id":job["id"], "table":table["slug"], "count":len(targets), "change":change, "sample":sample, "status":"planned"})
}

func (s *Server) ownedBatchJob(ctx context.Context, r *http.Request, idParam string, kind string) (map[string]any, bool) {
	app, _, err := s.appForRequest(ctx, r); if err != nil { return nil, false }
	job, err := s.PB.Get(ctx, "batch_jobs", pathID(r, idParam)); id := who(r)
	if err != nil || job["tenant_id"] != id.Tenant["id"] || job["app_id"] != app["id"] || job["user_id"] != id.User["id"] || (stringValue(asMap(job["plan"])["kind"]) == "import") != (kind == "import") { return nil, false }
	return job, true
}

func (s *Server) getBatchPlan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r); defer cancel(); job, ok := s.ownedBatchJob(ctx, r, "jobId", "batch"); if !ok { writeError(w, 404, "批量计划不存在"); return }
	plan := asMap(job["plan"]); writeJSON(w, 200, map[string]any{"id":job["id"], "status":job["status"], "count":len(anySlice(plan["targets"])), "change":plan["change"], "result":job["result"]})
}

func (s *Server) commitBatchPlan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute); defer cancel(); job, ok := s.ownedBatchJob(ctx, r, "jobId", "batch"); if !ok { writeError(w, 404, "批量计划不存在"); return }
	unlock := lockJob(stringValue(job["id"])); defer unlock(); job, _ = s.PB.Get(ctx, "batch_jobs", stringValue(job["id"]))
	input := mapBody(r); if input["confirm"] != true || input["plan_id"] != job["id"] { writeError(w, 400, "请确认当前批量计划"); return }
	if job["status"] == "completed" || job["status"] == "partial" { writeJSON(w, 200, map[string]any{"status":job["status"], "result":job["result"]}); return }
	if job["status"] == "running" && time.Since(parseTime(job["updated"])) < 2*time.Minute { writeError(w, 409, "计划正在执行"); return }
	if job["status"] != "planned" && job["status"] != "running" { writeError(w, 409, "计划已经失效"); return }
	if job["status"] == "planned" && time.Since(parseTime(job["created"])) > 15*time.Minute { writeError(w, 409, "计划已过期，请重新预览"); return }
	id := who(r); app, err := s.PB.Get(ctx, "apps", stringValue(job["app_id"])); if err != nil || app["tenant_id"] != id.Tenant["id"] || boolValue(app["archived"]) || !s.appCanBatch(ctx, app, id) { writeError(w, 403, "没有批量修改权限"); return }
	plan := asMap(job["plan"]); table, err := s.PB.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "slug = "+pbFilterString(stringValue(plan["table"]))))
	if err != nil || findField(asSliceMap(table["fields"]), stringValue(asMap(plan["change"])["field"])) == nil { writeError(w, 409, "表结构已变化"); return }
	_, _ = s.PB.Update(ctx, "batch_jobs", stringValue(job["id"]), map[string]any{"status":"running"})
	result := map[string]any{"updated":0, "conflicted":0, "failed":0, "items":[]map[string]any{}}
	items := result["items"].([]map[string]any)
	for _, target := range asSliceMap(plan["targets"]) {
		freshApp, e := s.PB.Get(ctx, "apps", stringValue(app["id"])); if e != nil || boolValue(freshApp["archived"]) || !s.appCanBatch(ctx, freshApp, id) { result["failed"] = intValue(result["failed"])+1; items = append(items, map[string]any{"id":target["id"], "status":"permission_changed"}); break }
		row, e := s.PB.Get(ctx, stringValue(table["pb_collection"]), stringValue(target["id"])); if e != nil || row["tenant_id"] != id.Tenant["id"] || row["app_id"] != app["id"] || row["updated"] != target["updated_at"] { result["conflicted"] = intValue(result["conflicted"])+1; items = append(items, map[string]any{"id":target["id"], "status":"conflict"}); continue }
		change := asMap(plan["change"]); data := map[string]any{stringValue(change["field"]):change["value"]}
		_, e = s.PB.UpdateBusiness(ctx, stringValue(table["pb_collection"]), stringValue(row["id"]), data, stringValue(target["updated_at"]), stringValue(id.User["id"]), "batch")
		if e != nil { if pe, ok := e.(interface{ Error() string }); ok && strings.Contains(pe.Error(), "conflict") { result["conflicted"] = intValue(result["conflicted"])+1; items = append(items, map[string]any{"id":target["id"], "status":"conflict"}) } else { result["failed"] = intValue(result["failed"])+1; items = append(items, map[string]any{"id":target["id"], "status":"failed"}) }; continue }
		updated, e := s.PB.Get(ctx, stringValue(table["pb_collection"]), stringValue(row["id"])); if e == nil { s.processRecordAutomation(ctx, stringValue(id.Tenant["id"]), stringValue(app["id"]), stringValue(table["slug"]), "updated", row, updated) }
		result["updated"] = intValue(result["updated"])+1; items = append(items, map[string]any{"id":target["id"], "status":"updated"})
		result["items"] = items; _, _ = s.PB.Update(ctx, "batch_jobs", stringValue(job["id"]), map[string]any{"result":result})
	}
	result["items"] = items; status := "completed"; if intValue(result["failed"]) > 0 || intValue(result["conflicted"]) > 0 { status = "partial" }
	_, err = s.PB.Update(ctx, "batch_jobs", stringValue(job["id"]), map[string]any{"status":status, "result":result}); if err != nil { writeError(w, 503, "批量结果保存失败"); return }
	writeJSON(w, 200, map[string]any{"status":status, "result":result})
}

func (s *Server) createImportPlan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r); defer cancel(); input := mapBody(r); app, table, err := s.operationTable(ctx, r, input)
	if err != nil { writeError(w, 404, "数据表不存在"); return }; id := who(r)
	if !s.appCanBatch(ctx, app, id) || boolValue(app["archived"]) { writeError(w, 403, "导入需要批量写入权限"); return }
	rows, ok := input["rows"].([]any); if !ok || len(rows) < 1 || len(rows) > 100 { writeError(w, 400, "导入需要真实数据表及 1–100 行已映射的数据"); return }
	raw, _ := json.Marshal(rows); if len(raw) > 500000 { writeError(w, 400, "导入计划超过大小限制"); return }
	fields := asSliceMap(table["fields"]); errors := []map[string]any{}
	for i, value := range rows { row, ok := value.(map[string]any); if !ok { errors = append(errors, map[string]any{"row":i+1,"error":"每行必须是对象"}); continue }; msg := validateData(row, fields, false); if msg == "" { msg = validateRelations(ctx, s.PB, row, fields, stringValue(app["id"]), stringValue(id.Tenant["id"])) }; if msg != "" { errors = append(errors, map[string]any{"row":i+1,"error":msg}) } }
	if len(errors) > 0 { writeJSON(w, 400, map[string]any{"error":"导入校验失败，没有写入记录", "errors":errors}); return }
	plan := map[string]any{"kind":"import", "table":table["slug"], "fields":fields, "rows":rows, "targets":[]any{}}
	job, err := s.PB.Create(ctx, "batch_jobs", map[string]any{"tenant_id":id.Tenant["id"], "app_id":app["id"], "user_id":id.User["id"], "status":"planned", "plan":plan})
	if err != nil { writeError(w, 503, "导入计划创建失败"); return }
	sample := rows; if len(sample)>10 { sample=sample[:10] }; writeJSON(w, 201, map[string]any{"plan_id":job["id"], "table":table["slug"], "count":len(rows), "sample":sample, "status":"planned", "note":"只新增，不自动合并；请审阅后确认。计划 15 分钟内有效。"})
}

func (s *Server) getImportPlan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r); defer cancel(); job, ok := s.ownedBatchJob(ctx, r, "planId", "import"); if !ok { writeError(w, 404, "导入计划不存在"); return }; plan:=asMap(job["plan"]); writeJSON(w, 200, map[string]any{"id":job["id"],"status":job["status"],"count":len(anySlice(plan["rows"])),"result":job["result"]})
}

func (s *Server) commitImportPlan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute); defer cancel(); job, ok := s.ownedBatchJob(ctx, r, "planId", "import"); if !ok { writeError(w, 404, "导入计划不存在"); return }
	unlock:=lockJob(stringValue(job["id"]));defer unlock();job,_=s.PB.Get(ctx,"batch_jobs",stringValue(job["id"]))
	input:=mapBody(r);if input["confirm"]!=true||input["plan_id"]!=job["id"]{writeError(w,400,"请确认这个具体导入计划");return}
	if job["status"]=="completed"||job["status"]=="partial"{writeJSON(w,200,map[string]any{"status":job["status"],"result":job["result"]});return}
	if job["status"]!="planned"||time.Since(parseTime(job["created"]))>15*time.Minute{writeError(w,409,"计划已过期或执行中；不要重复导入，请检查已有结果");return}
	id:=who(r);app,err:=s.PB.Get(ctx,"apps",stringValue(job["app_id"]));if err!=nil||boolValue(app["archived"])||!s.appCanBatch(ctx,app,id){writeError(w,403,"导入需要批量写入权限");return}
	plan:=asMap(job["plan"]);table,err:=s.PB.Find(ctx,"app_collections",listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])),"app_id = "+pbFilterString(stringValue(app["id"])),"slug = "+pbFilterString(stringValue(plan["table"]))))
	if err!=nil||!equalJSON(table["fields"],plan["fields"]){writeError(w,409,"表结构已变化，请重新预览");return}
	_,_=s.PB.Update(ctx,"batch_jobs",stringValue(job["id"]),map[string]any{"status":"running"})
	result:=map[string]any{"created":0,"failed":0,"items":[]map[string]any{}};items:=result["items"].([]map[string]any)
	for i,raw:=range anySlice(plan["rows"]){fresh,e:=s.PB.Get(ctx,"apps",stringValue(app["id"]));if e!=nil||boolValue(fresh["archived"])||!s.appCanBatch(ctx,fresh,id){result["failed"]=intValue(result["failed"])+len(anySlice(plan["rows"]))-i;items=append(items,map[string]any{"row":i+1,"status":"permission_changed"});break}
		values:=asMap(raw);if msg:=validateData(values,asSliceMap(table["fields"]),false);msg!=""{result["failed"]=intValue(result["failed"])+1;items=append(items,map[string]any{"row":i+1,"status":"failed","error":msg});continue}
		if msg:=validateRelations(ctx,s.PB,values,asSliceMap(table["fields"]),stringValue(app["id"]),stringValue(id.Tenant["id"]));msg!=""{result["failed"]=intValue(result["failed"])+1;items=append(items,map[string]any{"row":i+1,"status":"failed","error":msg});continue}
		values["tenant_id"],values["app_id"]=id.Tenant["id"],app["id"];created,e:=s.PB.Create(ctx,stringValue(table["pb_collection"]),values);if e!=nil{result["failed"]=intValue(result["failed"])+1;items=append(items,map[string]any{"row":i+1,"status":"failed","error":"记录保存失败"})}else{result["created"]=intValue(result["created"])+1;items=append(items,map[string]any{"row":i+1,"id":created["id"],"status":"created"});s.processRecordAutomation(ctx,stringValue(id.Tenant["id"]),stringValue(app["id"]),stringValue(table["slug"]),"created",nil,created)}
		result["items"]=items;_,_=s.PB.Update(ctx,"batch_jobs",stringValue(job["id"]),map[string]any{"result":result})
	}
	result["items"]=items;status:="completed";if intValue(result["failed"])>0{status="partial"};_,_=s.PB.Update(ctx,"batch_jobs",stringValue(job["id"]),map[string]any{"status":status,"result":result});writeJSON(w,200,map[string]any{"status":status,"result":result})
}
