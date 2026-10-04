package httpapi

import "net/http"

// BackendCatalog is the finite, server-owned vocabulary available to builders.
// BackendSpec is exported from live records; it is not a second schema store.
func (s *Server) routesCatalog() {
	s.Mux.HandleFunc("GET /api/apps/{id}/backend/catalog", s.auth(s.getBackendCatalog))
	s.Mux.HandleFunc("GET /api/apps/{id}/backend/spec", s.auth(s.getBackendSpec))
}

func backendCatalog() map[string]any {
	return map[string]any{
		"schema_version": 1,
		"capabilities": []map[string]any{
			{"id": "collections.create", "purpose": "创建数据表及字段", "parameters": map[string]any{"name": "string", "slug": "string", "fields": "field[]"}, "impact": "schema"},
			{"id": "collections.update", "purpose": "修改数据表字段", "parameters": map[string]any{"table": "table_id", "fields": "field[]"}, "impact": "schema"},
			{"id": "records.query", "purpose": "查询真实记录", "parameters": map[string]any{"table": "table_id", "page": "integer", "search": "string", "sort": "string"}, "impact": "read"},
			{"id": "business_actions.create", "purpose": "定义事务业务动作", "parameters": map[string]any{"name": "string", "definition": "action_definition"}, "impact": "write"},
			{"id": "workflows.configure", "purpose": "配置状态流转", "parameters": map[string]any{"name": "string", "definition": "workflow_definition"}, "impact": "write"},
			{"id": "automations.configure", "purpose": "配置停用的触发规则", "parameters": map[string]any{"name": "string", "definition": "automation_definition"}, "impact": "background_write"},
			{"id": "connectors.configure", "purpose": "配置受限 HTTPS 读取连接器", "parameters": map[string]any{"name": "string", "definition": "connector_definition"}, "impact": "external_read"},
			{"id": "collection_scripts.configure", "purpose": "配置声明式采集、去重和通知", "parameters": map[string]any{"name": "string", "definition": "collection_script_definition"}, "impact": "external_read+background_write"},
			{"id": "members.assign", "purpose": "授权真实成员访问应用", "parameters": map[string]any{"user_id": "workspace_member_id", "role": "app_role"}, "impact": "permissions"},
			{"id": "ui.compose", "purpose": "组合受控 json-render 页面", "parameters": map[string]any{"spec": "json_render_spec"}, "impact": "ui"},
		},
		"field_types":         []string{"text", "number", "bool", "date", "email", "url", "select", "relation", "member", "file"},
		"condition_operators": []string{"eq", "neq", "contains", "gt", "gte", "lt", "lte", "is_empty", "not_empty"},
		"statuses":            []string{"draft", "enabled", "paused", "archived"},
		"constraints":         map[string]any{"max_fields": 24, "no_scripts": true, "no_sql": true, "no_html": true, "no_cross_app_references": true},
	}
}

func (s *Server) getBackendCatalog(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	if _, _, err := s.appForRequest(ctx, r); err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	writeJSON(w, 200, backendCatalog())
}

func (s *Server) getBackendSpec(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	tables, err := s.appTables(ctx, app, stringValue(who(r).Tenant["id"]))
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	spec := map[string]any{"schema_version": 1, "app_id": app["id"], "tables": []any{}, "actions": []any{}, "workflows": []any{}, "automations": []any{}, "connectors": []any{}, "tasks": []any{}, "collection_scripts": []any{}}
	for _, table := range tables {
		spec["tables"] = append(spec["tables"].([]any), map[string]any{"id": table["id"], "logical_id": table["slug"], "name": table["name"], "fields": table["fields"]})
	}
	actions, err := s.PB.ListAll(ctx, "business_actions", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "status != \"archived\""), "-updated")
	if err != nil {
		writeError(w, 503, "后端规范暂不可用")
		return
	}
	for _, row := range actions {
		spec["actions"] = append(spec["actions"].([]any), publicBusinessAction(row))
	}
	workflows, err := s.PB.ListAll(ctx, "workflows", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "status != \"archived\""), "-updated")
	if err != nil {
		writeError(w, 503, "后端规范暂不可用")
		return
	}
	for _, row := range workflows {
		spec["workflows"] = append(spec["workflows"].([]any), map[string]any{"id": row["id"], "logical_id": row["id"], "name": row["name"], "definition": row["definition"], "status": row["status"], "revision": row["revision"]})
	}
	for _, collection := range []struct{ name, key string }{{"automation_rules", "automations"}, {"connectors", "connectors"}, {"miao_tasks", "tasks"}, {"collection_scripts", "collection_scripts"}} {
		rows, listErr := s.PB.ListAll(ctx, collection.name, listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "-updated")
		if listErr != nil { writeError(w, 503, "后端规范暂不可用"); return }
		for _, row := range rows {
			if row["status"] == "archived" { continue }
			spec[collection.key] = append(spec[collection.key].([]any), map[string]any{"id": row["id"], "logical_id": row["id"], "name": row["name"], "definition": row["definition"], "status": defaultString(stringValue(row["status"]), "draft"), "revision": row["revision"], "enabled": row["enabled"]})
		}
	}
	permissions, permErr := s.PB.ListAll(ctx, "app_members", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "")
	if permErr != nil { writeError(w, 503, "后端规范暂不可用"); return }
	spec["restricted"], spec["permissions"] = boolValue(app["restricted"]), []any{}
	for _, grant := range permissions { spec["permissions"] = append(spec["permissions"].([]any), map[string]any{"user_id": grant["user_id"], "role": grant["role"], "can_batch": grant["can_batch"]}) }
	writeJSON(w, 200, spec)
}
