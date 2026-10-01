package httpapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/tans/miao/internal/pocketbase"
)

var reservedAppFields = map[string]bool{"id": true, "created": true, "updated": true, "collectionid": true, "collectionname": true, "app_id": true, "tenant_id": true}
var allowedFieldTypes = map[string]bool{"text": true, "number": true, "bool": true, "date": true, "email": true, "url": true, "select": true, "relation": true, "file": true}
var slugReplace = regexp.MustCompile(`[^a-z0-9]+`)

func publicApp(app map[string]any) map[string]any {
	return map[string]any{"id": app["id"], "name": app["name"], "description": app["description"], "archived": boolValue(app["archived"]), "restricted": boolValue(app["restricted"]), "has_published_version": stringValue(app["published_version_id"]) != "", "permission": app["permission"], "created_at": app["created"], "updated_at": app["updated"]}
}
func publicTable(table map[string]any) map[string]any {
	return map[string]any{"id": table["id"], "name": table["name"], "slug": table["slug"], "fields": table["fields"], "created_at": table["created"]}
}

func (s *Server) routesApps() {
	s.Mux.HandleFunc("GET /api/apps", s.auth(s.listApps))
	s.Mux.HandleFunc("POST /api/apps", s.auth(s.createApp))
	s.Mux.HandleFunc("GET /api/apps/{id}", s.auth(s.getApp))
	s.Mux.HandleFunc("PATCH /api/apps/{id}", s.auth(s.updateApp))
	s.Mux.HandleFunc("DELETE /api/apps/{id}", s.auth(s.deleteApp))
	s.Mux.HandleFunc("GET /api/apps/{id}/collections", s.auth(s.listTables))
	s.Mux.HandleFunc("POST /api/apps/{id}/collections", s.auth(s.createTable))
	s.Mux.HandleFunc("PATCH /api/apps/{id}/collections/{slug}", s.auth(s.updateTable))
	s.Mux.HandleFunc("DELETE /api/apps/{id}/collections/{slug}", s.auth(s.deleteTable))
	s.Mux.HandleFunc("GET /api/apps/{id}/collections/{slug}/records", s.auth(s.listRecords))
	s.Mux.HandleFunc("POST /api/apps/{id}/collections/{slug}/records", s.auth(s.createRecord))
	s.Mux.HandleFunc("GET /api/apps/{id}/collections/{slug}/records/{recordId}", s.auth(s.getRecord))
	s.Mux.HandleFunc("PATCH /api/apps/{id}/collections/{slug}/records/{recordId}", s.auth(s.updateRecord))
	s.Mux.HandleFunc("DELETE /api/apps/{id}/collections/{slug}/records/{recordId}", s.auth(s.deleteRecord))
	s.Mux.HandleFunc("GET /api/apps/{id}/collections/{slug}/records/{recordId}/files/{fieldName}", s.auth(s.recordFile))
	s.Mux.HandleFunc("GET /api/apps/{id}/access", s.auth(s.getAppAccess))
	s.Mux.HandleFunc("PUT /api/apps/{id}/access", s.auth(s.updateAppAccess))
}

func (s *Server) listApps(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	archived := r.URL.Query().Get("archived") == "true"
	value := "false"
	if archived {
		value = "true"
	}
	rows, err := s.PB.ListAll(ctx, "apps", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "archived = "+value), "-updated")
	if err != nil {
		writeError(w, 503, "应用列表暂时不可用")
		return
	}
	out := []map[string]any{}
	for _, app := range rows {
		role := s.appPermission(ctx, app, id)
		if role != "" {
			app["permission"] = role
			out = append(out, publicApp(app))
		}
	}
	writeJSON(w, 200, out)
}
func (s *Server) createApp(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	input := mapBody(r)
	name := strings.TrimSpace(stringValue(input["name"]))
	if name == "" {
		writeError(w, 400, "请输入应用名称")
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, err := s.PB.Create(ctx, "apps", map[string]any{"tenant_id": id.Tenant["id"], "creator_id": id.User["id"], "name": clip(name, 160), "description": clip(stringValue(input["description"]), 4000)})
	if err != nil {
		writeError(w, 503, "应用创建失败")
		return
	}
	role := "owner"
	if id.Membership["role"] != "owner" {
		role = "publisher"
		_, _ = s.PB.Create(ctx, "app_members", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "user_id": id.User["id"], "role": role})
	}
	app["permission"] = role
	writeJSON(w, 201, publicApp(app))
}
func (s *Server) getApp(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	writeJSON(w, 200, publicApp(app))
}
func (s *Server) updateApp(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canManageAppRole(role) {
		writeError(w, 403, "你没有管理此应用的权限")
		return
	}
	input := mapBody(r)
	updates := map[string]any{}
	if raw, ok := input["name"]; ok {
		name := strings.TrimSpace(stringValue(raw))
		if name == "" {
			writeError(w, 400, "请输入应用名称")
			return
		}
		updates["name"] = clip(name, 160)
	}
	if raw, ok := input["description"]; ok {
		updates["description"] = clip(stringValue(raw), 4000)
	}
	if raw, ok := input["archived"]; ok {
		updates["archived"] = raw == true
	}
	if len(updates) == 0 {
		writeJSON(w, 200, publicApp(app))
		return
	}
	saved, err := s.PB.Update(ctx, "apps", stringValue(app["id"]), updates)
	if err != nil {
		writeError(w, 503, "应用更新失败")
		return
	}
	saved["permission"] = role
	writeJSON(w, 200, publicApp(saved))
}
func (s *Server) deleteApp(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if id.Membership["role"] != "owner" || id.Tenant["owner_id"] != id.User["id"] {
		writeError(w, 403, "只有工作区所有者可以调整应用访问权限")
		return
	}
	if mapBody(r)["confirm"] != true {
		writeError(w, 400, "删除应用会永久删除其中所有数据，请明确确认")
		return
	}
	tables := s.appTables(ctx, app, stringValue(id.Tenant["id"]))
	deleted := 0
	for _, table := range tables {
		rows, _, _, _ := s.PB.List(ctx, stringValue(table["pb_collection"]), "tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "", 1, 100000)
		deleted += len(rows)
		_ = s.PB.DeleteCollection(ctx, stringValue(table["pb_collection"]))
	}
	for _, name := range []string{"app_collections", "app_members", "app_versions", "miao_run_attempts", "miao_actions", "miao_runs", "miao_tasks", "app_files", "miao_record_changes", "agent_threads", "batch_jobs", "automation_notifications", "automation_runs", "automation_rules"} {
		rows, _ := s.PB.ListAll(ctx, name, listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "")
		for _, row := range rows {
			_ = s.PB.Delete(ctx, name, stringValue(row["id"]))
		}
	}
	if err := s.PB.Delete(ctx, "apps", stringValue(app["id"])); err != nil {
		writeError(w, 503, "应用删除失败")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "deleted_records": deleted})
}

func (s *Server) listTables(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	rows := s.appTables(ctx, app, stringValue(who(r).Tenant["id"]))
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, publicTable(row))
	}
	writeJSON(w, 200, out)
}
func normalizeAppFields(ctx context.Context, s *Server, app, tenant map[string]any, raw []any, old []map[string]any) ([]map[string]any, string) {
	if len(raw) < 1 || len(raw) > 24 {
		return nil, "数据表需要 1 到 24 个字段"
	}
	seen := map[string]bool{}
	out := []map[string]any{}
	for _, value := range raw {
		field := asMap(value)
		name := cleanAppSlug(stringValue(field["name"]))
		typ := defaultString(stringValue(field["type"]), "text")
		if name == "" || reservedAppFields[name] || seen[name] {
			return nil, "字段名无效或重复"
		}
		if !allowedFieldTypes[typ] {
			return nil, "暂不支持「" + typ + "」字段"
		}
		seen[name] = true
		item := map[string]any{"name": name, "label": clip(strings.TrimSpace(defaultString(stringValue(field["label"]), name)), 120), "type": typ, "required": boolValue(field["required"])}
		current := findField(old, name)
		if current != nil && current["type"] != typ {
			return nil, "字段「" + stringValue(current["label"]) + "」不能直接更改类型；请新增字段并迁移数据"
		}
		if typ == "select" {
			opts := uniqueStrings(field["options"], 40)
			if len(opts) < 2 {
				return nil, "字段「" + stringValue(item["label"]) + "」至少需要两个有效选项"
			}
			for _, opt := range opts {
				if len(opt) > 120 {
					return nil, "选项长度不能超过 120 个字符"
				}
			}
			item["options"] = opts
		}
		if typ == "relation" {
			target := defaultString(stringValue(field["target"]), stringValue(asMap(current)["target"]))
			meta, err := s.PB.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "slug = "+pbFilterString(target)))
			if err != nil {
				return nil, "关联字段必须选择当前应用中的数据表"
			}
			collection, err := s.PB.Collection(ctx, stringValue(meta["pb_collection"]))
			if err != nil {
				return nil, "关联数据表暂不可用"
			}
			item["target"], item["target_name"], item["target_collection_id"] = meta["slug"], meta["name"], collection["id"]
			if current != nil && stringValue(current["target"]) != target {
				return nil, "关联字段不能直接更改关联数据表"
			}
		}
		out = append(out, item)
	}
	return out, ""
}
func pbSchemaField(field map[string]any) map[string]any {
	typ := stringValue(field["type"])
	base := map[string]any{"name": field["name"], "type": typ, "required": boolValue(field["required"])}
	switch typ {
	case "select":
		base["maxSelect"], base["values"] = 1, field["options"]
	case "relation":
		base["maxSelect"], base["collectionId"], base["cascadeDelete"] = 1, field["target_collection_id"], false
	case "file":
		base["protected"], base["maxSelect"], base["maxSize"], base["mimeTypes"] = true, 1, 5*1024*1024, []string{"image/*", "application/pdf", "text/plain"}
	case "text":
		base["max"] = 10000
	}
	return base
}
func (s *Server) createTable(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canManageAppRole(role) {
		writeError(w, 403, "你没有管理此应用的权限")
		return
	}
	input := mapBody(r)
	name := strings.TrimSpace(stringValue(input["name"]))
	if name == "" {
		writeError(w, 400, "请输入数据表名称")
		return
	}
	fields, msg := normalizeAppFields(ctx, s, app, who(r).Tenant, anySlice(input["fields"]), nil)
	if msg != "" {
		writeError(w, 400, msg)
		return
	}
	slug := cleanAppSlug(defaultString(stringValue(input["slug"]), name))
	if slug == "" {
		slug = "table"
	}
	pbName := "app_" + stringValue(app["id"]) + "_" + slug
	schemaFields := []map[string]any{{"name": "created", "type": "autodate", "onCreate": true, "system": true}, {"name": "updated", "type": "autodate", "onCreate": true, "onUpdate": true, "system": true}, {"name": "app_id", "type": "text", "required": true, "max": 64}, {"name": "tenant_id", "type": "text", "required": true, "max": 64}}
	for _, field := range fields {
		schemaFields = append(schemaFields, pbSchemaField(field))
	}
	schema := map[string]any{"type": "base", "name": pbName, "listRule": nil, "viewRule": nil, "createRule": nil, "updateRule": nil, "deleteRule": nil, "fields": schemaFields}
	if _, err = s.PB.CreateCollection(ctx, schema); err != nil {
		writeError(w, 409, "数据表创建失败，请检查名称和字段")
		return
	}
	meta, err := s.PB.Create(ctx, "app_collections", map[string]any{"tenant_id": who(r).Tenant["id"], "app_id": app["id"], "name": clip(name, 160), "slug": slug, "pb_collection": pbName, "fields": fields})
	if err != nil {
		_ = s.PB.DeleteCollection(ctx, pbName)
		writeError(w, 503, "数据表创建失败")
		return
	}
	writeJSON(w, 201, publicTable(meta))
}

func (s *Server) publishedUIUses(ctx context.Context, app map[string]any, slug string, removed []map[string]any) bool {
	vid := stringValue(app["published_version_id"])
	if vid == "" {
		return false
	}
	version, err := s.PB.Get(ctx, "app_versions", vid)
	if err != nil {
		return false
	}
	definition := asMap(version["definition"])
	pages := asSliceMap(definition["pages"])
	for _, page := range pages {
		if stringValue(page["collection"]) != slug {
			continue
		}
		if len(removed) == 0 {
			return true
		}
		allowed := map[string]bool{}
		for _, field := range removed {
			allowed[stringValue(field["name"])] = true
		}
		for _, raw := range anySlice(page["fields"]) {
			f := asMap(raw)
			if allowed[stringValue(f["name"])] {
				return true
			}
		}
	}
	return false
}

func (s *Server) updateTable(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在")
		return
	}
	if !canManageAppRole(role) {
		writeError(w, 403, "你没有管理此应用的权限")
		return
	}
	meta, err := s.tableForRequest(ctx, r, app)
	if err != nil {
		writeError(w, 404, "数据表不存在")
		return
	}
	input := mapBody(r)
	updates := map[string]any{}
	if raw, ok := input["name"]; ok {
		name := strings.TrimSpace(stringValue(raw))
		if name == "" {
			writeError(w, 400, "请输入数据表名称")
			return
		}
		updates["name"] = clip(name, 160)
	}
	if raw, ok := input["fields"]; ok {
		old := asSliceMap(meta["fields"])
		fields, msg := normalizeAppFields(ctx, s, app, who(r).Tenant, anySlice(raw), old)
		if msg != "" {
			writeError(w, 400, msg)
			return
		}
		names := map[string]bool{}
		for _, field := range fields {
			names[stringValue(field["name"])] = true
		}
		removed := []map[string]any{}
		for _, field := range old {
			if !names[stringValue(field["name"])] {
				removed = append(removed, field)
			}
		}
		remove := stringSet(input["remove_fields"])
		for _, field := range removed {
			if !remove[stringValue(field["name"])] {
				writeError(w, 400, "要删除字段时请明确列出字段名")
				return
			}
		}
		if len(removed) > 0 && input["confirm_data_loss"] != true {
			writeError(w, 400, "删除字段会永久清除这些字段中的数据，请明确确认")
			return
		}
		if len(removed) > 0 && s.publishedUIUses(ctx, app, stringValue(meta["slug"]), removed) {
			writeError(w, 409, "这些字段正在当前已发布界面中使用。请先为界面创建并发布不再引用它们的新版本，再删除字段")
			return
		}
		schema, err := s.PB.Collection(ctx, stringValue(meta["pb_collection"]))
		if err != nil {
			writeError(w, 503, "数据表结构暂不可用")
			return
		}
		_, count, _, err := s.PB.List(ctx, stringValue(meta["pb_collection"]), listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "", 1, 1)
		if err != nil {
			writeError(w, 503, "记录检查暂不可用")
			return
		}
		schemaFields := []map[string]any{}
		for _, f := range asSliceMap(schema["fields"]) {
			name := stringValue(f["name"])
			if reservedAppFields[name] {
				schemaFields = append(schemaFields, f)
			}
		}
		for _, f := range fields {
			oldField := findField(old, stringValue(f["name"]))
			if boolValue(f["required"]) && oldField == nil && count > 0 {
				writeError(w, 400, "新增必填字段前，请先确保数据表没有现有记录")
				return
			}
			existing := map[string]any{}
			for _, schemaField := range asSliceMap(schema["fields"]) {
				if schemaField["name"] == f["name"] {
					existing = schemaField
					break
				}
			}
			if len(existing) > 0 {
				existing["required"] = f["required"]
				if f["type"] == "select" {
					existing["values"] = f["options"]
				}
				schemaFields = append(schemaFields, existing)
			} else {
				schemaFields = append(schemaFields, pbSchemaField(f))
			}
		}
		newSchema := map[string]any{"type": "base", "name": schema["name"], "listRule": nil, "viewRule": nil, "createRule": nil, "updateRule": nil, "deleteRule": nil, "fields": schemaFields, "indexes": schema["indexes"]}
		if _, err = s.PB.UpdateCollection(ctx, stringValue(meta["pb_collection"]), newSchema); err != nil {
			writeError(w, 409, "字段设置未能保存，请检查现有数据和值约束")
			return
		}
		updates["fields"] = fields
	}
	if len(updates) == 0 {
		writeJSON(w, 200, publicTable(meta))
		return
	}
	updated, err := s.PB.Update(ctx, "app_collections", stringValue(meta["id"]), updates)
	if err != nil {
		writeError(w, 503, "数据表设置保存失败")
		return
	}
	writeJSON(w, 200, publicTable(updated))
}

func (s *Server) deleteTable(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在")
		return
	}
	if !canManageAppRole(role) {
		writeError(w, 403, "你没有管理此应用的权限")
		return
	}
	meta, err := s.tableForRequest(ctx, r, app)
	if err != nil {
		writeError(w, 404, "数据表不存在")
		return
	}
	if mapBody(r)["confirm"] != true {
		writeError(w, 400, "删除数据表会永久删除其中所有记录，请明确确认")
		return
	}
	if s.publishedUIUses(ctx, app, stringValue(meta["slug"]), nil) {
		writeError(w, 409, "此数据表正在当前已发布界面中使用。请先为界面创建并发布引用其他数据表的新版本，再删除此表")
		return
	}
	_, count, _, _ := s.PB.List(ctx, stringValue(meta["pb_collection"]), "", "", 1, 1)
	if err = s.PB.DeleteCollection(ctx, stringValue(meta["pb_collection"])); err != nil {
		writeError(w, 503, "数据表删除失败")
		return
	}
	_ = s.PB.Delete(ctx, "app_collections", stringValue(meta["id"]))
	writeJSON(w, 200, map[string]any{"ok": true, "deleted_records": count})
}

func (s *Server) recordsContext(ctx context.Context, r *http.Request) (map[string]any, map[string]any, bool) {
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		return nil, nil, false
	}
	table, err := s.tableForRequest(ctx, r, app)
	return app, table, err == nil
}
func (s *Server) listRecords(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, table, ok := s.recordsContext(ctx, r)
	if !ok {
		writeError(w, 404, "数据表不存在")
		return
	}
	fields := asSliceMap(table["fields"])
	parts := []string{"tenant_id = " + pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = " + pbFilterString(stringValue(app["id"]))}
	search := clip(strings.TrimSpace(r.URL.Query().Get("search")), 120)
	if search != "" {
		alternatives := []string{}
		for _, field := range fields {
			if contains([]string{"text", "email", "url"}, field["type"]) {
				alternatives = append(alternatives, stringValue(field["name"])+" ~ "+pbFilterString(search))
			}
		}
		if len(alternatives) > 0 {
			parts = append(parts, "("+strings.Join(alternatives, " || ")+")")
		}
	}
	filterField, filterValue := r.URL.Query().Get("filterField"), r.URL.Query().Get("filterValue")
	if filterValue != "" {
		field := findField(fields, filterField)
		if field != nil {
			switch field["type"] {
			case "number":
				if _, e := fmt.Sscanf(filterValue, "%f", new(float64)); e != nil {
					writeError(w, 400, "筛选值必须是数字")
					return
				}
				parts = append(parts, filterField+" = "+filterValue)
			case "bool":
				if filterValue != "true" && filterValue != "false" {
					writeError(w, 400, "筛选值必须是布尔值")
					return
				}
				parts = append(parts, filterField+" = "+filterValue)
			default:
				parts = append(parts, filterField+" = "+pbFilterString(clip(filterValue, 200)))
			}
		}
	}
	sortName := r.URL.Query().Get("sort")
	allowed := map[string]bool{"created": true, "updated": true}
	for _, field := range fields {
		allowed[stringValue(field["name"])] = true
	}
	if !allowed[strings.TrimPrefix(sortName, "-")] {
		sortName = "-created"
	}
	page, per := queryInt(r, "page", 1, 1, 1000000), queryInt(r, "perPage", 25, 1, 100)
	rows, total, pages, err := s.PB.List(ctx, stringValue(table["pb_collection"]), listFilter(parts...), sortName, page, per)
	if err != nil {
		writeError(w, 503, "记录列表暂时不可用")
		return
	}
	out := []map[string]any{}
	for _, row := range rows {
		out = append(out, publicRecord(row))
	}
	writeJSON(w, 200, pageResult(out, page, per, total))
	_ = pages
}
func (s *Server) getRecord(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, table, ok := s.recordsContext(ctx, r)
	if !ok {
		writeError(w, 404, "数据表不存在")
		return
	}
	row, err := s.PB.Get(ctx, stringValue(table["pb_collection"]), pathID(r, "recordId"))
	id := who(r)
	if err != nil || row["tenant_id"] != id.Tenant["id"] || row["app_id"] != app["id"] {
		writeError(w, 404, "记录不存在")
		return
	}
	writeJSON(w, 200, publicRecord(row))
}

func decodeRecordFiles(raw any, values map[string]any, fields []map[string]any) ([]pocketbase.Upload, string) {
	files := asMap(raw)
	out := []pocketbase.Upload{}
	for name, item := range files {
		field := findField(fields, name)
		if field == nil || field["type"] != "file" {
			return nil, "字段「" + name + "」不是文件字段"
		}
		file := asMap(item)
		encoded := stringValue(file["base64"])
		encoded = strings.TrimPrefix(encoded, "data:"+stringValue(file["type"])+";base64,")
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(data) == 0 || len(data) > 5*1024*1024 {
			return nil, "附件大小或内容无效，单个附件不能超过 5 MB"
		}
		mime := defaultString(stringValue(file["type"]), "application/octet-stream")
		if !contains([]string{"image/png", "image/jpeg", "image/gif", "image/webp", "application/pdf", "text/plain"}, mime) {
			return nil, "附件类型不支持"
		}
		filename := strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._-", r) || r > 127 {
				return r
			}
			return '_'
		}, stringValue(file["name"]))
		filename = clip(strings.Trim(filename, "."), 120)
		if filename == "" {
			filename = "attachment"
		}
		out = append(out, pocketbase.Upload{Name: name, Filename: filename, ContentType: mime, Data: data})
	}
	return out, ""
}

func uploadedFileFields(files []pocketbase.Upload) map[string]bool {
	out := map[string]bool{}
	for _, file := range files {
		out[file.Name] = true
	}
	return out
}

func (s *Server) createRecord(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if role == "viewer" {
		writeError(w, 403, "你只有查看权限，不能修改此应用")
		return
	}
	table, err := s.tableForRequest(ctx, r, app)
	if err != nil {
		writeError(w, 404, "数据表不存在")
		return
	}
	input := mapBody(r)
	values := asMap(input["data"])
	if len(values) == 0 && input["data"] == nil {
		writeError(w, 400, "记录内容必须是对象")
		return
	}
	files, msg := decodeRecordFiles(input["files"], values, asSliceMap(table["fields"]))
	if msg != "" {
		writeError(w, 400, msg)
		return
	}
	if msg = validateDataWithFiles(values, asSliceMap(table["fields"]), false, uploadedFileFields(files)); msg != "" {
		writeError(w, 400, msg)
		return
	}
	id := who(r)
	if msg = validateRelations(ctx, s.PB, values, asSliceMap(table["fields"]), stringValue(app["id"]), stringValue(id.Tenant["id"])); msg != "" {
		writeError(w, 400, msg)
		return
	}
	values["app_id"], values["tenant_id"] = app["id"], id.Tenant["id"]
	var row map[string]any
	if len(files) > 0 {
		row, err = s.PB.UploadNew(ctx, stringValue(table["pb_collection"]), values, files)
	} else {
		row, err = s.PB.Create(ctx, stringValue(table["pb_collection"]), values)
	}
	if err != nil {
		writeError(w, 400, "记录保存失败，请检查字段值")
		return
	}
	s.processRecordAutomation(ctx, stringValue(id.Tenant["id"]), stringValue(app["id"]), stringValue(table["slug"]), "created", nil, row)
	writeJSON(w, 201, publicRecord(row))
}
func (s *Server) updateRecord(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if role == "viewer" {
		writeError(w, 403, "你只有查看权限，不能修改此应用")
		return
	}
	table, err := s.tableForRequest(ctx, r, app)
	if err != nil {
		writeError(w, 404, "数据表不存在")
		return
	}
	id := who(r)
	row, err := s.PB.Get(ctx, stringValue(table["pb_collection"]), pathID(r, "recordId"))
	if err != nil || row["tenant_id"] != id.Tenant["id"] || row["app_id"] != app["id"] {
		writeError(w, 404, "记录不存在")
		return
	}
	input := mapBody(r)
	values := asMap(input["data"])
	files, msg := decodeRecordFiles(input["files"], values, asSliceMap(table["fields"]))
	if msg != "" {
		writeError(w, 400, msg)
		return
	}
	fields := asSliceMap(table["fields"])
	if msg = validateDataWithFiles(values, fields, true, uploadedFileFields(files)); msg != "" {
		writeError(w, 400, msg)
		return
	}
	if msg = validateRelations(ctx, s.PB, values, fields, stringValue(app["id"]), stringValue(id.Tenant["id"])); msg != "" {
		writeError(w, 400, msg)
		return
	}
	expected := stringValue(input["expected_updated_at"])
	if expected != "" && expected != stringValue(row["updated"]) {
		writeError(w, 409, "记录已变化，请重新读取后操作")
		return
	}
	if expected == "" {
		expected = stringValue(row["updated"])
	}
	var saved map[string]any
	if len(files) > 0 {
		saved, err = s.PB.UploadBusiness(ctx, stringValue(table["pb_collection"]), stringValue(row["id"]), values, files, expected, stringValue(id.User["id"]), "interactive")
	} else {
		saved, err = s.PB.UpdateBusiness(ctx, stringValue(table["pb_collection"]), stringValue(row["id"]), values, expected, stringValue(id.User["id"]), "interactive")
	}
	if err != nil {
		if pbErr, ok := err.(*pocketbase.Error); ok && pbErr.Status == 409 {
			writeError(w, 409, "记录已变化，请重新读取后操作")
		} else {
			writeError(w, 503, "记录保存失败，请稍后重试")
		}
		return
	}
	s.processRecordAutomation(ctx, stringValue(id.Tenant["id"]), stringValue(app["id"]), stringValue(table["slug"]), "updated", row, saved)
	writeJSON(w, 200, publicRecord(saved))
}
func (s *Server) deleteRecord(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if role == "viewer" {
		writeError(w, 403, "你只有查看权限，不能修改此应用")
		return
	}
	table, err := s.tableForRequest(ctx, r, app)
	if err != nil {
		writeError(w, 404, "数据表不存在")
		return
	}
	row, err := s.PB.Get(ctx, stringValue(table["pb_collection"]), pathID(r, "recordId"))
	id := who(r)
	if err != nil || row["tenant_id"] != id.Tenant["id"] || row["app_id"] != app["id"] {
		writeError(w, 404, "记录不存在")
		return
	}
	if err = s.PB.Delete(ctx, stringValue(table["pb_collection"]), stringValue(row["id"])); err != nil {
		writeError(w, 503, "记录删除失败")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) getAppAccess(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	if !s.requireOwner(w, id) {
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, err := s.PB.Get(ctx, "apps", pathID(r, "id"))
	if err != nil || app["tenant_id"] != id.Tenant["id"] {
		writeError(w, 404, "应用不存在")
		return
	}
	members, _ := s.PB.ListAll(ctx, "tenant_members", "tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "")
	permissions, _ := s.PB.ListAll(ctx, "app_members", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "")
	assigned := map[string]map[string]any{}
	for _, item := range permissions {
		assigned[stringValue(item["user_id"])] = item
	}
	out := []map[string]any{}
	for _, membership := range members {
		if membership["role"] == "owner" {
			continue
		}
		user, e := s.PB.Get(ctx, "users", stringValue(membership["user_id"]))
		if e != nil {
			continue
		}
		p := assigned[stringValue(user["id"])]
		item := map[string]any{"id": user["id"], "email": user["email"], "name": user["name"], "workspace_role": membership["role"], "app_role": "", "can_batch": false}
		if p != nil {
			item["app_role"], item["can_batch"] = p["role"], p["can_batch"]
		}
		out = append(out, item)
	}
	writeJSON(w, 200, map[string]any{"restricted": boolValue(app["restricted"]), "members": out})
}
func (s *Server) updateAppAccess(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	if !s.requireOwner(w, id) {
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, err := s.PB.Get(ctx, "apps", pathID(r, "id"))
	if err != nil || app["tenant_id"] != id.Tenant["id"] {
		writeError(w, 404, "应用不存在")
		return
	}
	input := mapBody(r)
	restricted, ok := input["restricted"].(bool)
	permissions, ok2 := input["permissions"].([]any)
	if !ok || !ok2 || len(permissions) > 500 {
		writeError(w, 400, "访问权限设置无效")
		return
	}
	members, _ := s.PB.ListAll(ctx, "tenant_members", "tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "")
	valid := map[string]bool{}
	for _, m := range members {
		if m["role"] != "owner" {
			valid[stringValue(m["user_id"])] = true
		}
	}
	normalized := map[string]map[string]any{}
	for _, raw := range permissions {
		p := asMap(raw)
		uid, role := stringValue(p["user_id"]), stringValue(p["role"])
		if !valid[uid] || !contains([]string{"viewer", "editor", "manager", "publisher"}, role) {
			writeError(w, 400, "权限成员或角色无效")
			return
		}
		normalized[uid] = map[string]any{"role": role, "can_batch": boolValue(p["can_batch"]) && role != "viewer"}
	}
	current, _ := s.PB.ListAll(ctx, "app_members", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "")
	for _, p := range current {
		_ = s.PB.Delete(ctx, "app_members", stringValue(p["id"]))
	}
	for uid, p := range normalized {
		_, _ = s.PB.Create(ctx, "app_members", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "user_id": uid, "role": p["role"], "can_batch": p["can_batch"]})
	}
	_, err = s.PB.Update(ctx, "apps", stringValue(app["id"]), map[string]any{"restricted": restricted})
	if err != nil {
		writeError(w, 503, "访问权限保存失败")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) recordFile(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, table, ok := s.recordsContext(ctx, r)
	if !ok {
		writeError(w, 404, "附件不存在")
		return
	}
	field := findField(asSliceMap(table["fields"]), pathID(r, "fieldName"))
	if field == nil || field["type"] != "file" {
		writeError(w, 404, "附件不存在")
		return
	}
	row, err := s.PB.Get(ctx, stringValue(table["pb_collection"]), pathID(r, "recordId"))
	id := who(r)
	if err != nil || row["app_id"] != app["id"] || row["tenant_id"] != id.Tenant["id"] || stringValue(row[stringValue(field["name"])]) == "" {
		writeError(w, 404, "附件不存在")
		return
	}
	data, contentType, filename, err := s.PB.ProtectedFile(ctx, stringValue(table["pb_collection"]), stringValue(row["id"]), stringValue(row[stringValue(field["name"])]))
	if err != nil {
		writeError(w, 502, "附件暂时无法读取")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "inline; filename*=UTF-8''"+urlQueryEscape(filename))
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
func urlQueryEscape(value string) string { return url.PathEscape(value) }
