package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/tans/miao/internal/pocketbase"
)

// Business actions are the generic domain layer between tables and UI. A
// definition contains guarded, transactional record steps; it does not know
// whether the app is a catalog, CRM, order system, or something else.
func (s *Server) routesBusinessActions() {
	s.Mux.HandleFunc("GET /api/apps/{id}/actions", s.auth(s.listBusinessActions))
	s.Mux.HandleFunc("POST /api/apps/{id}/actions", s.auth(s.createBusinessAction))
	s.Mux.HandleFunc("PATCH /api/apps/{id}/actions/{actionId}", s.auth(s.updateBusinessAction))
	s.Mux.HandleFunc("POST /api/apps/{id}/actions/{actionId}/enable", s.auth(s.enableBusinessAction))
	s.Mux.HandleFunc("POST /api/apps/{id}/actions/{actionId}/execute", s.auth(s.executeBusinessAction))
}

func publicBusinessAction(row map[string]any) map[string]any {
	return map[string]any{"id": row["id"], "name": row["name"], "description": row["description"], "definition": row["definition"], "status": row["status"], "revision": row["revision"], "pause_reason": defaultString(stringValue(row["pause_reason"]), "")}
}

type pocketbaseError struct {
	status  int
	message string
}

func (e *pocketbaseError) Error() string { return e.message }

func (s *Server) listBusinessActions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	rows, err := s.PB.ListAll(ctx, "business_actions", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "status != \"archived\""), "-updated")
	if err != nil {
		writeError(w, 503, "业务动作暂不可用")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, publicBusinessAction(row))
	}
	writeJSON(w, 200, out)
}

func (s *Server) createBusinessAction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canManageAppRole(s.appPermission(ctx, app, who(r))) {
		writeError(w, 403, "你没有管理此应用的权限")
		return
	}
	input := mapBody(r)
	definition, msg := s.normalizeBusinessAction(ctx, app, input["definition"])
	if msg != "" {
		writeError(w, 400, msg)
		return
	}
	name := strings.TrimSpace(stringValue(input["name"]))
	if name == "" {
		writeError(w, 400, "请输入业务动作名称")
		return
	}
	row, err := s.PB.Create(ctx, "business_actions", map[string]any{"tenant_id": who(r).Tenant["id"], "app_id": app["id"], "created_by": who(r).User["id"], "name": clip(name, 160), "description": clip(stringValue(input["description"]), 1000), "definition": definition, "status": "draft", "revision": 1})
	if err != nil {
		writeError(w, 503, "业务动作创建失败")
		return
	}
	writeJSON(w, 201, publicBusinessAction(row))
}

func (s *Server) actionForRequest(ctx context.Context, r *http.Request, manage bool) (map[string]any, map[string]any, error) {
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		return nil, nil, err
	}
	if manage && !canManageAppRole(s.appPermission(ctx, app, who(r))) {
		return nil, nil, &pocketbaseError{403, "你没有管理此应用的权限"}
	}
	action, err := s.PB.Get(ctx, "business_actions", pathID(r, "actionId"))
	if err != nil || action["tenant_id"] != who(r).Tenant["id"] || action["app_id"] != app["id"] {
		return nil, nil, &pocketbaseError{404, "业务动作不存在"}
	}
	return app, action, nil
}

func (s *Server) updateBusinessAction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	_, action, err := s.actionForRequest(ctx, r, true)
	if err != nil {
		writeError(w, errStatus(err), err.Error())
		return
	}
	input := mapBody(r)
	definition, msg := s.normalizeBusinessAction(ctx, mustApp(ctx, s, r), input["definition"])
	if msg != "" {
		writeError(w, 400, msg)
		return
	}
	updates := map[string]any{"definition": definition, "revision": intValue(action["revision"]) + 1, "status": "draft", "pause_reason": ""}
	if _, ok := input["name"]; ok {
		updates["name"] = clip(strings.TrimSpace(stringValue(input["name"])), 160)
	}
	if _, ok := input["description"]; ok {
		updates["description"] = clip(stringValue(input["description"]), 1000)
	}
	saved, err := s.PB.Update(ctx, "business_actions", stringValue(action["id"]), updates)
	if err != nil {
		writeError(w, 503, "业务动作更新失败")
		return
	}
	writeJSON(w, 200, publicBusinessAction(saved))
}

func (s *Server) enableBusinessAction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	_, action, err := s.actionForRequest(ctx, r, true)
	if err != nil {
		writeError(w, errStatus(err), err.Error())
		return
	}
	if mapBody(r)["confirm"] != true {
		writeError(w, 400, "需要确认启用业务动作")
		return
	}
	status := "enabled"
	if mapBody(r)["enabled"] == false {
		status = "paused"
	}
	saved, err := s.PB.Update(ctx, "business_actions", stringValue(action["id"]), map[string]any{"status": status, "pause_reason": ""})
	if err != nil {
		writeError(w, 503, "业务动作状态更新失败")
		return
	}
	writeJSON(w, 200, publicBusinessAction(saved))
}

func errStatus(err error) int {
	if e, ok := err.(*pocketbaseError); ok {
		return e.status
	}
	return 404
}
func mustApp(ctx context.Context, s *Server, r *http.Request) map[string]any {
	app, _, _ := s.appForRequest(ctx, r)
	return app
}

func (s *Server) executeBusinessAction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, action, err := s.actionForRequest(ctx, r, false)
	if err != nil {
		writeError(w, errStatus(err), err.Error())
		return
	}
	if action["status"] != "enabled" {
		writeError(w, 409, "业务动作尚未启用")
		return
	}
	input := mapBody(r)
	definition := asMap(action["definition"])
	key := strings.TrimSpace(stringValue(input["idempotency_key"]))
	if key == "" || len(key) > 160 {
		writeError(w, 400, "必须提供不超过 160 个字符的 idempotency_key")
		return
	}
	if previous, findErr := s.PB.Find(ctx, "business_action_runs", listFilter("action_id = "+pbFilterString(stringValue(action["id"])), "idempotency_key = "+pbFilterString(key))); findErr == nil {
		writeJSON(w, 200, previous["result"])
		return
	}
	results, err := s.executeActionSteps(ctx, who(r), app, action, definition, asMap(input["input"]))
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	result := map[string]any{"status": "completed", "action": action["id"], "revision": action["revision"], "steps": results}
	if _, err := s.PB.Create(ctx, "business_action_runs", map[string]any{"tenant_id": who(r).Tenant["id"], "app_id": app["id"], "action_id": action["id"], "revision": action["revision"], "idempotency_key": key, "status": "completed", "result": result}); err != nil {
		if previous, findErr := s.PB.Find(ctx, "business_action_runs", listFilter("action_id = "+pbFilterString(stringValue(action["id"])), "idempotency_key = "+pbFilterString(key))); findErr == nil {
			writeJSON(w, 200, previous["result"])
			return
		}
		writeError(w, 503, "业务动作回执保存失败")
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) normalizeBusinessAction(ctx context.Context, app map[string]any, raw any) (map[string]any, string) {
	definition := asMap(raw)
	if len(definition) == 0 {
		return nil, "业务动作定义必须是对象"
	}
	steps := asSliceMap(definition["steps"])
	if len(steps) < 1 || len(steps) > 20 {
		return nil, "业务动作需要 1–20 个步骤"
	}
	tables, err := s.PB.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(app["tenant_id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "")
	if err != nil {
		return nil, "应用数据表暂不可用"
	}
	seen := map[string]bool{}
	conditions := asSliceMap(definition["conditions"])
	if len(conditions) > 40 {
		return nil, "业务动作条件不能超过 40 个"
	}
	normalized := []map[string]any{}
	for _, step := range steps {
		id := stringValue(step["id"])
		op := stringValue(step["operation"])
		tableName := stringValue(step["table"])
		if id == "" || seen[id] || !containsString([]string{"create", "update"}, op) {
			return nil, "步骤标识或操作无效"
		}
		seen[id] = true
		var table map[string]any
		for _, candidate := range tables {
			if candidate["slug"] == tableName {
				table = candidate
				break
			}
		}
		if table == nil {
			return nil, "步骤引用的数据表不存在"
		}
		data := asMap(step["data"])
		if len(data) == 0 {
			return nil, "步骤必须提供字段数据"
		}
		for field, value := range data {
			schema := findField(asSliceMap(table["fields"]), field)
			if schema == nil || schema["type"] == "file" {
				return nil, fmt.Sprintf("步骤字段无效：%s", field)
			}
			if !isActionValue(value) {
				return nil, "步骤字段值必须是 JSON 标量或引用"
			}
		}
		if op == "update" && stringValue(step["record_id"]) == "" {
			return nil, "更新步骤必须提供 record_id"
		}
		if op == "create" && step["record_id"] != nil {
			return nil, "创建步骤不能提供 record_id"
		}
		if op == "update" && !isActionReference(stringValue(step["record_id"])) && stringValue(step["record_id"]) == "" {
			return nil, "更新步骤的 record_id 无效"
		}
		normalized = append(normalized, map[string]any{"id": id, "operation": op, "table": tableName, "record_id": step["record_id"], "expected_updated_at": step["expected_updated_at"], "data": data})
	}
	for _, condition := range conditions {
		if stringValue(condition["table"]) == "" || stringValue(condition["record_id"]) == "" || stringValue(condition["field"]) == "" || !containsString([]string{"eq", "neq", "empty", "not_empty"}, stringValue(condition["op"])) {
			return nil, "业务动作条件无效"
		}
		var table map[string]any
		for _, candidate := range tables {
			if candidate["slug"] == condition["table"] {
				table = candidate
				break
			}
		}
		if table == nil || findField(asSliceMap(table["fields"]), stringValue(condition["field"])) == nil {
			return nil, "业务动作条件引用的字段不存在"
		}
	}
	return map[string]any{"conditions": conditions, "steps": normalized}, ""
}

func isActionValue(value any) bool {
	switch value.(type) {
	case nil, bool, float64, string:
		return true
	default:
		return false
	}
}

func (s *Server) executeActionSteps(ctx context.Context, id identity, app, action, definition, input map[string]any) ([]map[string]any, error) {
	steps := asSliceMap(definition["steps"])
	results := make([]map[string]any, 0, len(steps))
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		if _, err := s.authorizeWrite(ctx, tx, id.actor(stringValue(app["id"]), "interactive"), false); err != nil {
			return err
		}
		for _, condition := range asSliceMap(definition["conditions"]) {
			table, err := tx.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "slug = "+pbFilterString(stringValue(condition["table"]))))
			if err != nil {
				return err
			}
			conditionRecordID := resolveActionString(stringValue(condition["record_id"]), input)
			row, err := tx.Get(ctx, stringValue(table["pb_collection"]), conditionRecordID)
			if err != nil {
				return err
			}
			value := row[stringValue(condition["field"])]
			matches := actionConditionMatches(value, stringValue(condition["op"]), condition["value"])
			if !matches {
				return businessError(409, "业务动作前置条件不满足")
			}
		}
		for _, step := range steps {
			table, err := tx.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "slug = "+pbFilterString(stringValue(step["table"]))))
			if err != nil {
				return err
			}
			fields := asSliceMap(table["fields"])
			data := resolveActionInput(asMap(step["data"]), input)
			recordID := resolveActionString(stringValue(step["record_id"]), input)
			expectedUpdated := resolveActionString(stringValue(step["expected_updated_at"]), input)
			if msg := validateDataWithFiles(data, fields, stringValue(step["record_id"]) != "", nil); msg != "" {
				return businessError(400, msg)
			}
			if msg := validateRelations(ctx, tx, data, fields, stringValue(app["id"]), stringValue(id.Tenant["id"])); msg != "" {
				return businessError(400, msg)
			}
			if stringValue(step["operation"]) == "update" {
				row, err := tx.Get(ctx, stringValue(table["pb_collection"]), recordID)
				if err != nil {
					return err
				}
				if row["tenant_id"] != id.Tenant["id"] || row["app_id"] != app["id"] {
					return businessError(404, "动作目标记录不存在")
				}
				if expectedUpdated == "" {
					return businessError(400, "更新步骤必须提供 expected_updated_at")
				}
			}
			if stringValue(step["operation"]) == "create" {
				data["tenant_id"], data["app_id"] = id.Tenant["id"], app["id"]
			}
			saved, err := tx.UploadBusiness(ctx, stringValue(table["pb_collection"]), recordID, data, nil, expectedUpdated, stringValue(id.User["id"]), "interactive")
			if err != nil {
				return err
			}
			results = append(results, map[string]any{"id": step["id"], "operation": step["operation"], "table": step["table"], "record": publicRecord(saved)})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func isActionReference(value string) bool { return strings.HasPrefix(value, "$") && len(value) > 1 }

func resolveActionString(value string, input map[string]any) string {
	if isActionReference(value) {
		return stringValue(input[strings.TrimPrefix(value, "$")])
	}
	return value
}

func actionConditionMatches(value any, op string, expected any) bool {
	empty := value == nil || value == ""
	switch op {
	case "empty":
		return empty
	case "not_empty":
		return !empty
	case "eq":
		return equalJSON(value, expected)
	case "neq":
		return !equalJSON(value, expected)
	default:
		return false
	}
}

func resolveActionInput(data, input map[string]any) map[string]any {
	resolved := map[string]any{}
	for name, value := range data {
		if ref, ok := value.(string); ok && strings.HasPrefix(ref, "$") {
			resolved[name] = input[strings.TrimPrefix(ref, "$")]
		} else {
			resolved[name] = value
		}
	}
	return resolved
}
