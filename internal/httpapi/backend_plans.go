package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tans/miao/internal/pocketbase"
)

const backendPlanLifetime = 30 * time.Minute
const backendPlanMaxOperations = 20

type backendPlanCandidate struct {
	ID          string         `json:"id"`
	Capability  string         `json:"capability"`
	Description string         `json:"description"`
	Impact      string         `json:"impact"`
	Available   bool           `json:"available"`
	Reason      string         `json:"reason,omitempty"`
	Bound       map[string]any `json:"-"`
}

type backendPlanOperation struct {
	ID           string         `json:"id"`
	CandidateID  string         `json:"candidate_id"`
	Capability   string         `json:"capability"`
	Impact       string         `json:"impact"`
	Dependencies []string       `json:"dependencies"`
	Input        map[string]any `json:"input"`
}

func backendOpaqueID(prefix, scope string, parts ...string) string {
	value := scope
	for _, part := range parts {
		value += "\x00" + part
	}
	sum := sha256.Sum256([]byte(value))
	return prefix + hex.EncodeToString(sum[:12])
}

func (s *Server) backendPlanContext(ctx context.Context, r *http.Request, manage bool) (map[string]any, string, error) {
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		return nil, "", &pocketbaseError{404, "应用不存在或你没有访问权限"}
	}
	if manage && !canManageAppRole(role) {
		return nil, "", &pocketbaseError{403, "你没有管理此应用的权限"}
	}
	return app, role, nil
}

func optionalBackendRows(ctx context.Context, pb *pocketbase.Client, collection, filter, sortBy string) ([]map[string]any, error) {
	if _, err := pb.Collection(ctx, collection); err != nil {
		if isMissing(err) {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	return pb.ListAll(ctx, collection, filter, sortBy)
}

func (s *Server) backendPlanSnapshot(ctx context.Context, app map[string]any, tenantID string) (map[string]any, string, error) {
	appID := stringValue(app["id"])
	filter := listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(appID))
	tables, err := s.PB.ListAll(ctx, "app_collections", filter, "created")
	if err != nil {
		return nil, "", err
	}
	resources := map[string]any{"app": map[string]any{"id": app["id"], "tenant_id": app["tenant_id"], "restricted": app["restricted"], "archived": app["archived"]}, "tables": tables}
	for _, item := range []struct{ collection, key string }{
		{"business_actions", "actions"}, {"workflows", "workflows"}, {"automation_rules", "automations"},
		{"miao_tasks", "tasks"}, {"collection_scripts", "collection_scripts"},
		{"app_members", "permissions"},
	} {
		rows, listErr := optionalBackendRows(ctx, s.PB, item.collection, filter, "created")
		if listErr != nil {
			return nil, "", listErr
		}
		resources[item.key] = rows
	}
	members, err := s.PB.ListAll(ctx, "tenant_members", "tenant_id = "+pbFilterString(tenantID), "created")
	if err != nil {
		return nil, "", err
	}
	resources["tenant_members"] = members
	canonical, err := json.Marshal(resources)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(canonical)
	return resources, hex.EncodeToString(sum[:]), nil
}

func (s *Server) backendPlanCandidates(ctx context.Context, app map[string]any, role string, actor identity) ([]backendPlanCandidate, map[string]any, []map[string]any, error) {
	tenantID, appID := stringValue(app["tenant_id"]), stringValue(app["id"])
	resources, _, err := s.backendPlanSnapshot(ctx, app, tenantID)
	if err != nil {
		return nil, nil, nil, err
	}
	tables := asSliceMap(resources["tables"])
	members := asSliceMap(resources["tenant_members"])
	permissions := asSliceMap(resources["permissions"])
	byUser := map[string]map[string]any{}
	for _, permission := range permissions {
		byUser[stringValue(permission["user_id"])] = permission
	}
	candidates := []backendPlanCandidate{}
	add := func(capability, impact, label, boundType, boundID string, extra ...string) {
		bound := map[string]any{}
		if boundType != "" {
			bound[boundType] = boundID
		}
		parts := []string{capability, boundType, boundID}
		parts = append(parts, extra...)
		candidates = append(candidates, backendPlanCandidate{ID: backendOpaqueID("bp1-", tenantID+"\x00"+appID, parts...), Capability: capability, Description: label, Impact: impact, Available: true, Bound: bound})
	}
	if canManageAppRole(role) {
		add("collections.create", "schema", "Create table and fields", "", "")
		add("business_actions.create", "write", "Create business action", "", "")
		add("workflows.configure", "write", "Create workflow", "", "")
		if canPublishAppRole(role) {
			add("collection_scripts.configure", "external_read+background_write", "Create collection script", "", "")
			add("connectors.configure", "external_read", "Create connector", "", "")
		}
		for _, group := range []struct{ key, capability string }{{"actions", "business_actions.update"}, {"workflows", "workflows.update"}} {
			for _, resource := range asSliceMap(resources[group.key]) {
				if resource["status"] != "archived" {
					add(group.capability, "write", "Revise "+stringValue(resource["name"]), "resource_id", stringValue(resource["id"]))
				}
			}
		}
		for _, table := range tables {
			add("collections.update", "schema", "Update fields on "+stringValue(table["name"]), "table_id", stringValue(table["id"]))
		}
	}
	tenant, err := s.PB.Get(ctx, "tenants", tenantID)
	if err != nil {
		return nil, nil, nil, err
	}
	if stringValue(tenant["owner_id"]) == stringValue(actor.User["id"]) && actor.Tenant["id"] == tenantID && actor.Membership["role"] == "owner" {
		for _, member := range members {
			userID := stringValue(member["user_id"])
			if member["role"] == "owner" {
				continue
			}
			user, getErr := s.PB.Get(ctx, "users", userID)
			if getErr == nil && !boolValue(user["disabled"]) {
				candidates = append(candidates, backendPlanCandidate{ID: backendOpaqueID("bp1-", tenantID+"\x00"+appID, "members.assign", userID), Capability: "members.assign", Description: "Set access for " + defaultString(stringValue(user["name"]), stringValue(user["email"])), Impact: "permissions", Available: true, Bound: map[string]any{"user_id": userID}})
			}
		}
	}
	unsupported := []map[string]any{
		{"capability": "automations.configure", "available": false, "reason": "Automation plan execution is unavailable"},
		{"capability": "tasks.configure", "available": false, "reason": "Task plan execution is unavailable"},
		{"capability": "ui.compose", "available": false, "reason": "UI composition is unavailable in backend plans"},
	}
	refs := map[string]any{"tables": []map[string]any{}, "members": []map[string]any{}}
	refTables := []map[string]any{}
	for _, table := range tables {
		tableID := stringValue(table["id"])
		fields := []map[string]any{}
		for _, field := range asSliceMap(table["fields"]) {
			fields = append(fields, map[string]any{"name": field["name"], "label": field["label"], "type": field["type"], "reference_id": backendOpaqueID("ref-", tenantID+"\x00"+appID, "field", tableID, stringValue(field["name"]))})
		}
		refTables = append(refTables, map[string]any{"id": tableID, "reference_id": backendOpaqueID("ref-", tenantID+"\x00"+appID, "table", tableID), "logical_id": table["slug"], "name": table["name"], "fields": fields})
	}
	refs["tables"] = refTables
	refMembers := []map[string]any{}
	for _, member := range members {
		if member["role"] == "owner" {
			continue
		}
		user, getErr := s.PB.Get(ctx, "users", stringValue(member["user_id"]))
		if getErr != nil || boolValue(user["disabled"]) {
			continue
		}
		permission := byUser[stringValue(user["id"])]
		item := map[string]any{"user_id": user["id"], "reference_id": backendOpaqueID("ref-", tenantID+"\x00"+appID, "member", stringValue(user["id"])), "name": defaultString(stringValue(user["name"]), stringValue(user["email"])), "role": "", "can_batch": false}
		if permission != nil {
			item["role"], item["can_batch"] = permission["role"], permission["can_batch"]
		}
		refMembers = append(refMembers, item)
	}
	refs["members"] = refMembers
	for _, key := range []string{"actions", "workflows", "automations", "tasks", "collection_scripts", "permissions"} {
		refs[key] = resources[key]
	}
	return candidates, refs, unsupported, nil
}

func (s *Server) listBackendPlanCandidates(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.backendPlanContext(ctx, r, false)
	if err != nil {
		writeError(w, errStatus(err), err.Error())
		return
	}
	candidates, resources, unsupported, err := s.backendPlanCandidates(ctx, app, role, who(r))
	if err != nil {
		writeError(w, 503, "后端候选暂不可用")
		return
	}
	writeJSON(w, 200, map[string]any{"candidates": candidates, "resources": resources, "unsupported_capabilities": unsupported})
}

func backendCandidateIndex(candidates []backendPlanCandidate) map[string]backendPlanCandidate {
	out := make(map[string]backendPlanCandidate, len(candidates))
	for _, candidate := range candidates {
		if candidate.Available {
			out[candidate.ID] = candidate
		}
	}
	return out
}

func backendTableRef(raw any, tables []map[string]any, tenantID, appID string) (map[string]any, bool) {
	ref := stringValue(raw)
	for _, table := range tables {
		id := stringValue(table["id"])
		if ref == backendOpaqueID("ref-", tenantID+"\x00"+appID, "table", id) {
			return table, true
		}
	}
	return nil, false
}

func backendFieldRef(raw any, tables []map[string]any, tenantID, appID string) (map[string]any, map[string]any, bool) {
	ref := stringValue(raw)
	for _, table := range tables {
		for _, field := range asSliceMap(table["fields"]) {
			if ref == backendOpaqueID("ref-", tenantID+"\x00"+appID, "field", stringValue(table["id"]), stringValue(field["name"])) {
				return table, field, true
			}
		}
	}
	return nil, nil, false
}

func mapSliceAny(rows []map[string]any) []any {
	out := make([]any, len(rows))
	for i, row := range rows {
		out[i] = row
	}
	return out
}

func (s *Server) normalizePlanOperation(ctx context.Context, app map[string]any, candidate backendPlanCandidate, input map[string]any, tables []map[string]any, operationIndex int, tableCandidateID string) (map[string]any, []string, string) {
	input = cloneAnyMap(input)
	tenantID, appID := stringValue(app["tenant_id"]), stringValue(app["id"])
	dependencies := []string{}
	resolveTable := func(ref any) (map[string]any, bool) {
		if stringValue(ref) == tableCandidateID && tableCandidateID != "" {
			for _, table := range tables {
				if table["__plan_candidate_id"] == tableCandidateID {
					return table, true
				}
			}
			return nil, false
		}
		return backendTableRef(ref, tables, tenantID, appID)
	}
	switch candidate.Capability {
	case "collections.create":
		name := strings.TrimSpace(stringValue(input["name"]))
		if name == "" {
			return nil, nil, "请输入数据表名称"
		}
		slug := cleanAppSlug(defaultString(stringValue(input["slug"]), name))
		if slug == "" || len(slug) > 60 {
			return nil, nil, "数据表标识无效"
		}
		for _, table := range tables {
			if table["slug"] == slug {
				return nil, nil, "数据表标识已存在"
			}
		}
		fields := asSliceMap(input["fields"])
		for _, field := range fields {
			if stringValue(field["type"]) == "relation" {
				target, ok := backendTableRef(field["target_ref"], tables, tenantID, appID)
				if !ok {
					return nil, nil, "关联字段必须选择当前应用候选数据表"
				}
				field["target"] = target["slug"]
				delete(field, "target_ref")
			}
		}
		normalized, message := normalizeAppFields(ctx, s, app, map[string]any{"id": tenantID}, mapSliceAny(fields), nil)
		if message != "" {
			return nil, nil, message
		}
		input["name"], input["slug"], input["fields"] = clip(name, 160), slug, normalized
		tables = append(tables, map[string]any{"id": "planned-" + fmt.Sprint(operationIndex), "slug": slug, "name": name, "fields": normalized, "__plan_candidate_id": candidate.ID})
	case "collections.update":
		var target map[string]any
		for _, table := range tables {
			if stringValue(table["id"]) == stringValue(candidate.Bound["table_id"]) {
				target = table
				break
			}
		}
		if target == nil {
			return nil, nil, "候选数据表已失效"
		}
		fields, message := normalizeAppFields(ctx, s, app, map[string]any{"id": tenantID}, anySlice(input["fields"]), asSliceMap(target["fields"]))
		if message != "" {
			return nil, nil, message
		}
		input["fields"] = fields
		if name, supplied := input["name"]; supplied {
			value := strings.TrimSpace(stringValue(name))
			if value == "" {
				return nil, nil, "请输入数据表名称"
			}
			input["name"] = clip(value, 160)
		}
	case "business_actions.create", "business_actions.update":
		if candidate.Capability == "business_actions.update" {
			if message := s.normalizePlanResourceBaseline(ctx, app, candidate, input, "business_actions"); message != "" {
				return nil, nil, message
			}
		}
		if strings.TrimSpace(stringValue(input["name"])) == "" {
			return nil, nil, "请输入业务动作名称"
		}
		definition := asMap(input["definition"])
		steps := asSliceMap(definition["steps"])
		if len(steps) == 0 || len(steps) > 20 {
			return nil, nil, "业务动作需要 1–20 个步骤"
		}
		for _, step := range steps {
			table, ok := resolveTable(step["table_ref"])
			if !ok {
				return nil, nil, "业务动作步骤必须引用候选数据表"
			}
			tableRef := stringValue(step["table_ref"])
			if table["__plan_candidate_id"] == tableCandidateID && tableCandidateID != "" {
				dependencies = append(dependencies, fmt.Sprintf("op-%02d", operationIndex-1))
			}
			step["table"], step["__backend_table_ref"] = table["slug"], tableRef
			delete(step, "table_ref")
			resolvedData, fieldRefs := map[string]any{}, map[string]string{}
			for fieldRef, value := range asMap(step["data"]) {
				if table["__plan_candidate_id"] == tableCandidateID && tableCandidateID != "" {
					if findField(asSliceMap(table["fields"]), fieldRef) == nil {
						return nil, nil, "业务动作步骤引用的字段不存在"
					}
					resolvedData[fieldRef] = value
					continue
				}
				fieldTable, field, valid := backendFieldRef(fieldRef, tables, tenantID, appID)
				if !valid || fieldTable["id"] != table["id"] {
					return nil, nil, "业务动作字段必须引用步骤数据表中的候选字段"
				}
				name := stringValue(field["name"])
				resolvedData[name], fieldRefs[name] = value, fieldRef
			}
			step["data"], step["__backend_field_refs"] = resolvedData, fieldRefs
		}
		if _, message := normalizeBusinessActionForTables(definition, tables); message != "" {
			return nil, nil, message
		}
		input["definition"] = definition
	case "connectors.configure":
		if strings.TrimSpace(stringValue(input["name"])) == "" {
			return nil, nil, "请输入连接器名称"
		}
		definition, message := normalizeConnectorDefinition(input["definition"])
		if message != "" {
			return nil, nil, message
		}
		input["definition"] = definition
	case "collection_scripts.configure":
		if strings.TrimSpace(stringValue(input["name"])) == "" {
			return nil, nil, "请输入采集脚本名称"
		}
		definition := asMap(input["definition"])
		source := asMap(definition["source"])
		if stringValue(source["__backend_connector_ref"]) == "" {
			return nil, nil, "采集脚本连接器必须引用当前应用候选"
		}
		target := asMap(definition["target"])
		if stringValue(target["__backend_table_ref"]) == "" {
			return nil, nil, "采集脚本目标表必须引用当前应用候选"
		}
		resolved, message := resolveCollectionScriptPlanDefinition(ctx, s.PB, app, definition)
		if message != "" {
			return nil, nil, message
		}
		if _, message := normalizeCollectionScriptDefinitionWithConnector(ctx, s.PB, tenantID, appID, resolved, true); message != "" {
			return nil, nil, message
		}
		input["definition"] = definition
	case "workflows.configure", "workflows.update":
		if candidate.Capability == "workflows.update" {
			if message := s.normalizePlanResourceBaseline(ctx, app, candidate, input, "workflows"); message != "" {
				return nil, nil, message
			}
		}
		if strings.TrimSpace(stringValue(input["name"])) == "" {
			return nil, nil, "请输入流程名称"
		}
		definition := asMap(input["definition"])
		table, ok := resolveTable(definition["table_ref"])
		if !ok {
			return nil, nil, "流程必须引用候选数据表"
		}
		fieldName := stringValue(definition["state_field"])
		if fieldRef := stringValue(definition["state_field_ref"]); fieldRef != "" {
			fieldTable, field, valid := backendFieldRef(fieldRef, tables, tenantID, appID)
			if !valid || fieldTable["id"] != table["id"] {
				return nil, nil, "流程状态字段必须引用此数据表中的候选字段"
			}
			fieldName = stringValue(field["name"])
			definition["__backend_state_field_ref"] = fieldRef
			delete(definition, "state_field_ref")
		}
		field := findField(asSliceMap(table["fields"]), fieldName)
		if field == nil || !containsString([]string{"select", "text"}, stringValue(field["type"])) {
			return nil, nil, "流程状态字段必须是选项或文本字段"
		}
		if table["__plan_candidate_id"] == tableCandidateID && tableCandidateID != "" {
			dependencies = append(dependencies, fmt.Sprintf("op-%02d", operationIndex-1))
		}
		states, transitions := asSliceMap(definition["states"]), asSliceMap(definition["transitions"])
		if len(states) < 2 || len(states) > 32 || len(transitions) < 1 || len(transitions) > 64 {
			return nil, nil, "流程状态或转换数量无效"
		}
		stateIDs, options := map[string]bool{}, map[string]bool{}
		for _, value := range anySlice(field["options"]) {
			options[stringValue(value)] = true
		}
		for _, state := range states {
			id, label := strings.TrimSpace(stringValue(state["id"])), strings.TrimSpace(stringValue(state["label"]))
			if id == "" || len(id) > 64 || label == "" || len([]rune(label)) > 120 || stateIDs[id] || stringValue(field["type"]) == "select" && !options[id] {
				return nil, nil, "流程状态无效或不在字段选项中"
			}
			stateIDs[id] = true
		}
		transitionIDs := map[string]bool{}
		for _, transition := range transitions {
			id, from, to := strings.TrimSpace(stringValue(transition["id"])), stringValue(transition["from"]), stringValue(transition["to"])
			label := strings.TrimSpace(stringValue(transition["label"]))
			if id == "" || len(id) > 64 || transitionIDs[id] || !stateIDs[from] || !stateIDs[to] || from == to || label == "" || len([]rune(label)) > 120 {
				return nil, nil, "流程状态转换无效"
			}
			transitionIDs[id] = true
		}
		definition["table"], definition["state_field"] = table["slug"], fieldName
		definition["__backend_table_ref"] = definition["table_ref"]
		delete(definition, "table_ref")
		input["definition"] = definition
	case "members.assign":
		role := stringValue(input["role"])
		if !containsString([]string{"viewer", "editor", "manager", "publisher", "remove"}, role) {
			return nil, nil, "权限角色无效"
		}
		input["can_batch"] = boolValue(input["can_batch"]) && appRole(role).canWrite()
	default:
		return nil, nil, "此能力当前不可用于计划"
	}
	return input, dependencies, ""
}

func cloneAnyMap(value map[string]any) map[string]any {
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}

type backendPlanInput struct {
	CandidateID string         `json:"candidate_id"`
	Input       map[string]any `json:"input"`
}

func (s *Server) createBackendPlan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	var request struct {
		Operations []backendPlanInput `json:"operations"`
	}
	if err := readJSON(r, &request); err != nil {
		writeError(w, 400, "计划内容无效")
		return
	}
	row, err := s.createBackendPlanForActor(ctx, who(r).actor(pathID(r, "id"), "interactive"), request.Operations, "")
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	writeJSON(w, 201, publicBackendPlan(row))
}

func publicBackendPlan(row map[string]any) map[string]any {
	return map[string]any{"id": row["id"], "app_id": row["app_id"], "revision": row["revision"], "baseline_hash": row["baseline_hash"], "expires_at": row["expires_at"], "status": row["status"], "operations": row["operations"], "dependency_order": row["dependency_order"], "impact": row["impact"], "receipt": row["receipt"], "created_at": row["created"], "updated_at": row["updated"]}
}

func (s *Server) ownedBackendPlan(ctx context.Context, r *http.Request, app map[string]any) (map[string]any, error) {
	row, err := s.PB.Get(ctx, "app_backend_plans", pathID(r, "planId"))
	if err != nil || row["tenant_id"] != who(r).Tenant["id"] || row["app_id"] != app["id"] || row["user_id"] != who(r).User["id"] {
		return nil, &pocketbaseError{404, "计划不存在"}
	}
	return row, nil
}

func (s *Server) getBackendPlan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.backendPlanContext(ctx, r, false)
	if err != nil {
		writeError(w, errStatus(err), err.Error())
		return
	}
	plan, err := s.ownedBackendPlan(ctx, r, app)
	if err != nil {
		writeError(w, errStatus(err), err.Error())
		return
	}
	writeJSON(w, 200, publicBackendPlan(plan))
}

func (s *Server) applyBackendPlan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	request := mapBody(r)
	if request["confirm"] != true {
		writeError(w, 400, "需要明确确认应用计划")
		return
	}
	plan, err := s.applyBackendPlanForActor(ctx, who(r).actor(pathID(r, "id"), "interactive"), pathID(r, "planId"), intValue(request["expected_revision"]))
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	writeJSON(w, 200, publicBackendPlan(plan))
}

func (s *Server) resolveApplyInput(ctx context.Context, app map[string]any, candidate backendPlanCandidate, input map[string]any, operations, completed []any) (map[string]any, string) {
	data, err := json.Marshal(input)
	if err != nil {
		return nil, "计划输入无效"
	}
	copy := map[string]any{}
	if json.Unmarshal(data, &copy) != nil {
		return nil, "计划输入无效"
	}
	if !containsString([]string{"business_actions.create", "business_actions.update", "workflows.configure", "workflows.update", "collection_scripts.configure"}, candidate.Capability) {
		return copy, ""
	}
	resources, _, err := s.backendPlanSnapshot(ctx, app, stringValue(app["tenant_id"]))
	if err != nil {
		return nil, "应用数据表暂不可用"
	}
	tables := asSliceMap(resources["tables"])
	tenantID, appID := stringValue(app["tenant_id"]), stringValue(app["id"])
	resolve := func(ref string) (map[string]any, bool) {
		if table, ok := backendTableRef(ref, tables, tenantID, appID); ok {
			return table, true
		}
		step := asMap(completedResourceMap(completed)[ref])
		result := asMap(step["result"])
		if step["capability"] != "collections.create" {
			return nil, false
		}
		for _, table := range tables {
			if table["id"] == result["id"] && table["slug"] == result["slug"] {
				return table, true
			}
		}
		return nil, false
	}
	definition := asMap(copy["definition"])
	if candidate.Capability == "workflows.configure" || candidate.Capability == "workflows.update" {
		table, ok := resolve(stringValue(definition["__backend_table_ref"]))
		if !ok {
			return nil, "流程数据表候选已失效或依赖尚未完成"
		}
		definition["table"] = table["slug"]
		if ref := stringValue(definition["__backend_state_field_ref"]); ref != "" {
			fieldTable, field, ok := backendFieldRef(ref, tables, tenantID, appID)
			if !ok || fieldTable["id"] != table["id"] {
				return nil, "流程状态字段引用已失效"
			}
			definition["state_field"] = field["name"]
		}
		delete(definition, "__backend_table_ref")
		delete(definition, "__backend_state_field_ref")
		validated, message := normalizeWorkflowForTables(definition, tables)
		copy["definition"] = validated
		return copy, message
	}
	if candidate.Capability == "collection_scripts.configure" {
		source := asMap(definition["source"])
		if stringValue(source["__backend_connector_ref"]) == "" {
			return nil, "采集脚本连接器引用已失效"
		}
		target := asMap(definition["target"])
		if stringValue(target["__backend_table_ref"]) == "" {
			return nil, "采集脚本目标表引用已失效"
		}
		copy["definition"] = definition
		return copy, ""
	}
	for _, step := range asSliceMap(definition["steps"]) {
		table, ok := resolve(stringValue(step["__backend_table_ref"]))
		if !ok {
			return nil, "动作数据表候选已失效或依赖尚未完成"
		}
		step["table"] = table["slug"]
		for name, raw := range asMap(step["__backend_field_refs"]) {
			fieldTable, field, ok := backendFieldRef(raw, tables, tenantID, appID)
			if !ok || fieldTable["id"] != table["id"] || field["name"] != name {
				return nil, "动作字段引用已失效"
			}
		}
		delete(step, "__backend_table_ref")
		delete(step, "__backend_field_refs")
	}
	validated, message := normalizeBusinessActionForTables(definition, tables)
	copy["definition"] = validated
	return copy, message
}

func completedResourceMap(completed []any) map[string]any {
	out := map[string]any{}
	for _, value := range completed {
		step := asMap(value)
		out[stringValue(step["candidate_id"])] = step
	}
	return out
}

func resolveCollectionScriptPlanDefinition(ctx context.Context, pb *pocketbase.Client, app map[string]any, definition map[string]any) (map[string]any, string) {
	definition = cloneAnyMap(definition)
	source := cloneAnyMap(asMap(definition["source"]))
	connectorRef := stringValue(source["__backend_connector_ref"])
	if connectorRef == "" {
		return nil, "采集脚本连接器引用已失效"
	}
	connectors, err := pb.ListAll(ctx, "connectors", listFilter("tenant_id = "+pbFilterString(stringValue(app["tenant_id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "status != \"archived\""), "created")
	if err != nil {
		return nil, "连接器暂不可用"
	}
	var connector map[string]any
	for _, row := range connectors {
		if backendOpaqueID("ref-", stringValue(app["tenant_id"])+"\x00"+stringValue(app["id"]), "connector", stringValue(row["id"])) == connectorRef {
			connector = row
			break
		}
	}
	if connector == nil {
		return nil, "采集脚本连接器候选已失效"
	}
	source["connector_id"] = connector["id"]
	delete(source, "__backend_connector_ref")
	definition["source"] = source
	target := cloneAnyMap(asMap(definition["target"]))
	tableRef := stringValue(target["__backend_table_ref"])
	if tableRef == "" {
		return nil, "采集脚本目标表引用已失效"
	}
	tables, err := pb.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(app["tenant_id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "created")
	if err != nil {
		return nil, "目标表暂不可用"
	}
	for _, table := range tables {
		if backendOpaqueID("ref-", stringValue(app["tenant_id"])+"\x00"+stringValue(app["id"]), "table", stringValue(table["id"])) == tableRef {
			target["table"] = table["slug"]
			delete(target, "__backend_table_ref")
			definition["target"] = target
			return definition, ""
		}
	}
	return nil, "采集脚本目标表候选已失效"
}
