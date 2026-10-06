package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/tans/miao/internal/pocketbase"
)

// A workflow is a reusable state machine attached to one user-defined table
// and state field. It deliberately has no industry-specific vocabulary.
func (s *Server) routesWorkflows() {
	s.Mux.HandleFunc("GET /api/apps/{id}/workflows", s.auth(s.listWorkflows))
	s.Mux.HandleFunc("POST /api/apps/{id}/workflows", s.auth(s.createWorkflow))
	s.Mux.HandleFunc("PATCH /api/apps/{id}/workflows/{workflowId}", s.auth(s.updateWorkflow))
	s.Mux.HandleFunc("POST /api/apps/{id}/workflows/{workflowId}/enable", s.auth(s.enableWorkflow))
	s.Mux.HandleFunc("POST /api/apps/{id}/workflows/{workflowId}/transition", s.auth(s.transitionWorkflow))
}

func publicWorkflow(row map[string]any) map[string]any {
	return map[string]any{"id": row["id"], "name": row["name"], "description": row["description"], "definition": row["definition"], "status": row["status"], "revision": row["revision"], "pause_reason": defaultString(stringValue(row["pause_reason"]), "")}
}

func (s *Server) workflowForRequest(ctx context.Context, r *http.Request, manage bool) (map[string]any, map[string]any, error) {
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		return nil, nil, &pocketbaseError{404, "应用不存在或你没有访问权限"}
	}
	if manage && !canManageAppRole(s.appPermission(ctx, app, who(r))) {
		return nil, nil, &pocketbaseError{403, "你没有管理此应用的权限"}
	}
	workflow, err := s.PB.Get(ctx, "workflows", pathID(r, "workflowId"))
	if err != nil || workflow["tenant_id"] != who(r).Tenant["id"] || workflow["app_id"] != app["id"] {
		return nil, nil, &pocketbaseError{404, "流程不存在"}
	}
	return app, workflow, nil
}

func (s *Server) listWorkflows(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	rows, err := s.PB.ListAll(ctx, "workflows", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "status != \"archived\""), "-updated")
	if err != nil {
		writeError(w, 503, "流程暂不可用")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, publicWorkflow(row))
	}
	writeJSON(w, 200, out)
}

func (s *Server) createWorkflow(w http.ResponseWriter, r *http.Request) {
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
	definition, msg := s.normalizeWorkflow(ctx, app, input["definition"])
	if msg != "" {
		writeError(w, 400, msg)
		return
	}
	name := strings.TrimSpace(stringValue(input["name"]))
	if name == "" {
		writeError(w, 400, "请输入流程名称")
		return
	}
	row, err := s.createBusinessConfiguration(ctx, who(r), app, "workflows", map[string]any{"tenant_id": who(r).Tenant["id"], "app_id": app["id"], "created_by": who(r).User["id"], "name": clip(name, 160), "description": clip(stringValue(input["description"]), 1000), "definition": definition, "status": "draft", "revision": 1})
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	writeJSON(w, 201, publicWorkflow(row))
}

func (s *Server) updateWorkflow(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, workflow, err := s.workflowForRequest(ctx, r, true)
	if err != nil {
		writeError(w, errStatus(err), err.Error())
		return
	}
	input := mapBody(r)
	if expected := intValue(input["expected_revision"]); expected != 0 && expected != intValue(workflow["revision"]) {
		writeError(w, 409, "流程版本已变化，请重新读取后修改")
		return
	}
	definition, msg := s.normalizeWorkflow(ctx, app, input["definition"])
	if msg != "" {
		writeError(w, 400, msg)
		return
	}
	updates := map[string]any{"definition": definition, "revision": intValue(workflow["revision"]) + 1, "status": "draft", "pause_reason": ""}
	if _, ok := input["name"]; ok {
		updates["name"] = clip(strings.TrimSpace(stringValue(input["name"])), 160)
	}
	if _, ok := input["description"]; ok {
		updates["description"] = clip(stringValue(input["description"]), 1000)
	}
	saved, err := s.updateBusinessConfiguration(ctx, who(r), app, workflow, "workflows", updates)
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	writeJSON(w, 200, publicWorkflow(saved))
}

func (s *Server) enableWorkflow(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, workflow, err := s.workflowForRequest(ctx, r, true)
	if err != nil {
		writeError(w, errStatus(err), err.Error())
		return
	}
	input := mapBody(r)
	if input["confirm"] != true {
		writeError(w, 400, "需要确认启用流程")
		return
	}
	if expected := intValue(input["expected_revision"]); expected != 0 && expected != intValue(workflow["revision"]) {
		writeError(w, 409, "流程版本已变化，请重新读取后操作")
		return
	}
	status := "enabled"
	if input["enabled"] == false {
		status = "paused"
	}
	saved, err := s.updateBusinessConfiguration(ctx, who(r), app, workflow, "workflows", map[string]any{"status": status, "pause_reason": ""})
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	writeJSON(w, 200, publicWorkflow(saved))
}

func (s *Server) normalizeWorkflow(ctx context.Context, app map[string]any, raw any) (map[string]any, string) {
	tables, err := s.PB.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(app["tenant_id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "")
	if err != nil {
		return nil, "应用数据表暂不可用"
	}
	return normalizeWorkflowForTables(raw, tables)
}

func normalizeWorkflowForTables(raw any, tables []map[string]any) (map[string]any, string) {
	definition := asMap(raw)
	if len(definition) == 0 {
		return nil, "流程定义必须是对象"
	}
	tableName, stateFieldName := stringValue(definition["table"]), stringValue(definition["state_field"])
	var table map[string]any
	for _, candidate := range tables {
		if candidate["slug"] == tableName {
			table = candidate
		}
	}
	if table == nil {
		return nil, "流程引用的数据表不存在"
	}
	field := findField(asSliceMap(table["fields"]), stateFieldName)
	if field == nil || !containsString([]string{"select", "text"}, stringValue(field["type"])) {
		return nil, "流程状态字段必须是选项或文本字段"
	}
	states := asSliceMap(definition["states"])
	if len(states) < 2 || len(states) > 32 {
		return nil, "流程需要 2–32 个状态"
	}
	stateIDs := map[string]bool{}
	normalizedStates := make([]map[string]any, 0, len(states))
	options := map[string]bool{}
	for _, rawState := range anySlice(field["options"]) {
		options[stringValue(rawState)] = true
	}
	for _, state := range states {
		id := strings.TrimSpace(stringValue(state["id"]))
		label := strings.TrimSpace(stringValue(state["label"]))
		if id == "" || len(id) > 64 || label == "" || len([]rune(label)) > 120 || stateIDs[id] || stringValue(field["type"]) == "select" && !options[id] {
			return nil, "流程状态无效或不在字段选项中"
		}
		stateIDs[id] = true
		normalizedStates = append(normalizedStates, map[string]any{"id": id, "label": label})
	}
	transitions := asSliceMap(definition["transitions"])
	if len(transitions) < 1 || len(transitions) > 64 {
		return nil, "流程需要 1–64 个状态转换"
	}
	transitionIDs := map[string]bool{}
	normalizedTransitions := make([]map[string]any, 0, len(transitions))
	for _, transition := range transitions {
		id := strings.TrimSpace(stringValue(transition["id"]))
		from, to := stringValue(transition["from"]), stringValue(transition["to"])
		label := strings.TrimSpace(stringValue(transition["label"]))
		if id == "" || len(id) > 64 || transitionIDs[id] || !stateIDs[from] || !stateIDs[to] || from == to || label == "" || len([]rune(label)) > 120 {
			return nil, "流程状态转换无效"
		}
		transitionIDs[id] = true
		normalizedTransitions = append(normalizedTransitions, map[string]any{"id": id, "label": label, "from": from, "to": to})
	}
	return map[string]any{"table": tableName, "state_field": stateFieldName, "states": normalizedStates, "transitions": normalizedTransitions}, ""
}

func (s *Server) transitionWorkflow(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, workflow, err := s.workflowForRequest(ctx, r, false)
	if err != nil {
		writeError(w, errStatus(err), err.Error())
		return
	}
	if workflow["status"] != "enabled" {
		writeError(w, 409, "流程尚未启用")
		return
	}
	input := mapBody(r)
	result, err := s.executeWorkflowTransition(ctx, who(r), app, workflow, input, "interactive", nil)
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) executeWorkflowTransition(ctx context.Context, id identity, app, workflow, input map[string]any, source string, guard func(*pocketbase.Client) error) (map[string]any, error) {
	transitionID, recordID := stringValue(input["transition_id"]), stringValue(input["record_id"])
	key, expectedUpdated := strings.TrimSpace(stringValue(input["idempotency_key"])), stringValue(input["expected_updated_at"])
	if transitionID == "" || recordID == "" || key == "" || len(key) > 160 || expectedUpdated == "" {
		return nil, businessError(400, "必须提供 transition_id、record_id、expected_updated_at 和 idempotency_key")
	}
	definition := asMap(workflow["definition"])
	var transition map[string]any
	for _, candidate := range asSliceMap(definition["transitions"]) {
		if stringValue(candidate["id"]) == transitionID {
			transition = candidate
			break
		}
	}
	if transition == nil {
		return nil, businessError(400, "流程转换不存在")
	}
	stateField := stringValue(definition["state_field"])
	result := map[string]any{}
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		if _, authErr := s.authorizeWrite(ctx, tx, id.actor(stringValue(app["id"]), source), false); authErr != nil {
			return authErr
		}
		fresh, err := tx.Get(ctx, "workflows", stringValue(workflow["id"]))
		if err != nil {
			return err
		}
		if fresh["tenant_id"] != id.Tenant["id"] || fresh["app_id"] != app["id"] || fresh["status"] != "enabled" || intValue(fresh["revision"]) != intValue(workflow["revision"]) || !equalJSON(fresh["definition"], definition) {
			return businessError(409, "流程权限、定义或修订已变化，请重新读取")
		}
		filter := listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "workflow_id = "+pbFilterString(stringValue(workflow["id"])), "transition_id = "+pbFilterString(transitionID), "record_id = "+pbFilterString(recordID), "idempotency_key = "+pbFilterString(key))
		if previous, findErr := tx.Find(ctx, "workflow_runs", filter); findErr == nil {
			result = asMap(previous["result"])
			return nil
		} else if !isMissing(findErr) {
			return findErr
		}
		if guard != nil {
			if err := guard(tx); err != nil {
				return err
			}
		}
		table, err := tx.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "slug = "+pbFilterString(stringValue(definition["table"]))))
		if err != nil {
			return err
		}
		row, getErr := tx.Get(ctx, stringValue(table["pb_collection"]), recordID)
		if getErr != nil || row["tenant_id"] != id.Tenant["id"] || row["app_id"] != app["id"] {
			return businessError(404, "目标记录不存在")
		}
		if stringValue(row[stateField]) != stringValue(transition["from"]) {
			return businessError(409, "记录当前状态不允许此转换")
		}
		saved, saveErr := tx.UploadBusiness(ctx, stringValue(table["pb_collection"]), recordID, map[string]any{stateField: transition["to"]}, nil, expectedUpdated, stringValue(id.User["id"]), source)
		if saveErr != nil {
			return saveErr
		}
		result = map[string]any{"status": "completed", "workflow": workflow["id"], "revision": workflow["revision"], "transition_id": transitionID, "record": publicRecord(saved)}
		_, saveErr = tx.Create(ctx, "workflow_runs", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "workflow_id": workflow["id"], "revision": workflow["revision"], "transition_id": transitionID, "record_id": recordID, "idempotency_key": key, "status": "completed", "result": result})
		return saveErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
