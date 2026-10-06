package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/tans/miao/internal/harness"
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

func (s *Server) routesBackendPlans() {
	s.Mux.HandleFunc("GET /api/apps/{id}/backend/candidates", s.auth(s.listBackendPlanCandidates))
	s.Mux.HandleFunc("POST /api/apps/{id}/backend/plans", s.auth(s.createBackendPlan))
	s.Mux.HandleFunc("GET /api/apps/{id}/backend/plans/{planId}", s.auth(s.getBackendPlan))
	s.Mux.HandleFunc("POST /api/apps/{id}/backend/plans/{planId}/apply", s.auth(s.applyBackendPlan))
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
		{"connectors", "connectors"}, {"miao_tasks", "tasks"}, {"collection_scripts", "collection_scripts"},
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
		{"capability": "connectors.configure", "available": false, "reason": "Connector plan execution is unavailable"},
		{"capability": "tasks.configure", "available": false, "reason": "Task plan execution is unavailable"},
		{"capability": "collection_scripts.configure", "available": false, "reason": "Collection script plan execution is unavailable"},
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
	for _, key := range []string{"actions", "workflows", "automations", "connectors", "tasks", "collection_scripts", "permissions"} {
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
	case "business_actions.create":
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
		input["definition"] = definition
	case "workflows.configure":
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

func (s *Server) createBackendPlan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.backendPlanContext(ctx, r, true)
	if err != nil {
		writeError(w, errStatus(err), err.Error())
		return
	}
	var request struct {
		Operations []struct {
			CandidateID string         `json:"candidate_id"`
			Input       map[string]any `json:"input"`
		} `json:"operations"`
	}
	if err := readJSON(r, &request); err != nil || len(request.Operations) == 0 || len(request.Operations) > backendPlanMaxOperations {
		writeError(w, 400, "计划需要 1–20 个操作")
		return
	}
	candidates, _, _, err := s.backendPlanCandidates(ctx, app, role, who(r))
	if err != nil {
		writeError(w, 503, "后端候选暂不可用")
		return
	}
	byID := backendCandidateIndex(candidates)
	resources, baseline, err := s.backendPlanSnapshot(ctx, app, stringValue(who(r).Tenant["id"]))
	if err != nil {
		writeError(w, 503, "应用基线暂不可用")
		return
	}
	tables, operations := asSliceMap(resources["tables"]), make([]backendPlanOperation, 0, len(request.Operations))
	impactSet, seenCandidates := map[string]bool{}, map[string]bool{}
	tableCandidateID := ""
	for index, requested := range request.Operations {
		candidate, ok := byID[requested.CandidateID]
		if !ok || seenCandidates[candidate.ID] {
			writeError(w, 400, "候选不存在、重复或不可用")
			return
		}
		seenCandidates[candidate.ID] = true
		if candidate.Capability == "collections.create" {
			tableCandidateID = candidate.ID
		}
		input, dependencies, message := s.normalizePlanOperation(ctx, app, candidate, requested.Input, tables, index+1, tableCandidateID)
		if message != "" {
			writeError(w, 400, message)
			return
		}
		if candidate.Capability == "collections.create" {
			tables = append(tables, map[string]any{"id": "planned-" + fmt.Sprint(index+1), "slug": input["slug"], "name": input["name"], "fields": input["fields"], "__plan_candidate_id": candidate.ID})
		}
		operations = append(operations, backendPlanOperation{ID: fmt.Sprintf("op-%02d", index+1), CandidateID: candidate.ID, Capability: candidate.Capability, Impact: candidate.Impact, Dependencies: dependencies, Input: input})
		impactSet[candidate.Impact] = true
	}
	impact := make([]string, 0, len(impactSet))
	for value := range impactSet {
		impact = append(impact, value)
	}
	sort.Strings(impact)
	order := make([]string, 0, len(operations))
	operationValues := make([]any, 0, len(operations))
	for _, operation := range operations {
		order = append(order, operation.ID)
		operationValues = append(operationValues, map[string]any{"id": operation.ID, "candidate_id": operation.CandidateID, "capability": operation.Capability, "impact": operation.Impact, "dependencies": operation.Dependencies, "input": operation.Input})
	}
	impactValues := make([]any, 0, len(impact))
	for _, value := range impact {
		impactValues = append(impactValues, value)
	}
	expires := time.Now().UTC().Add(backendPlanLifetime).Format(time.RFC3339Nano)
	initialReceipt := map[string]any{"status": "draft", "completed_steps": []any{}, "remaining_steps": operationValues, "resource_ids": map[string]any{}}
	row, err := s.PB.Create(ctx, "app_backend_plans", map[string]any{"tenant_id": who(r).Tenant["id"], "app_id": app["id"], "user_id": who(r).User["id"], "status": "draft", "revision": 1, "baseline_hash": baseline, "expires_at": expires, "operations": operationValues, "dependency_order": order, "impact": impactValues, "receipt": initialReceipt})
	if err != nil {
		var validationErr *pocketbase.Error
		if errors.As(err, &validationErr) {
			s.Logger.Error("backend plan save failed", "error", err, "data", validationErr.Data)
		} else {
			s.Logger.Error("backend plan save failed", "error", err)
		}
		writeError(w, 503, "计划保存失败")
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
	app, role, err := s.backendPlanContext(ctx, r, true)
	if err != nil {
		writeError(w, errStatus(err), err.Error())
		return
	}
	plan, err := s.ownedBackendPlan(ctx, r, app)
	if err != nil {
		writeError(w, errStatus(err), err.Error())
		return
	}
	request := mapBody(r)
	if request["confirm"] != true {
		writeError(w, 400, "需要明确确认应用计划")
		return
	}
	if plan["status"] == "applied" || plan["status"] == "partial" {
		writeJSON(w, 200, publicBackendPlan(plan))
		return
	}
	if intValue(request["expected_revision"]) != intValue(plan["revision"]) {
		writeError(w, 409, "计划版本已变化，请重新读取")
		return
	}
	if !parseTime(plan["expires_at"]).After(time.Now()) {
		writeError(w, 409, "计划已过期，请重新创建")
		return
	}
	if plan["status"] != "draft" && plan["status"] != "applying" {
		writeError(w, 409, "计划当前状态不能应用")
		return
	}
	_, baseline, err := s.backendPlanSnapshot(ctx, app, stringValue(who(r).Tenant["id"]))
	if err != nil {
		writeError(w, 503, "应用基线暂不可用")
		return
	}
	if plan["status"] == "draft" && baseline != stringValue(plan["baseline_hash"]) {
		writeError(w, 409, "应用已变化，请重新读取并创建计划")
		return
	}
	candidates, _, _, err := s.backendPlanCandidates(ctx, app, role, who(r))
	if err != nil {
		writeError(w, 503, "后端候选暂不可用")
		return
	}
	byID := backendCandidateIndex(candidates)
	operations := asSliceMap(plan["operations"])
	for _, operation := range operations {
		if _, ok := byID[stringValue(operation["candidate_id"])]; !ok {
			writeError(w, 409, "计划候选或权限已失效")
			return
		}
	}
	receipt := asMap(plan["receipt"])
	completed := anySlice(receipt["completed_steps"])
	completedIDs := map[string]bool{}
	resourceIDs := asMap(receipt["resource_ids"])
	for _, item := range completed {
		completedIDs[stringValue(asMap(item)["operation_id"])] = true
	}
	if plan["status"] == "draft" {
		remaining := mapSliceAny(operations)
		receipt = map[string]any{"status": "applying", "completed_steps": completed, "remaining_steps": remaining, "resource_ids": resourceIDs}
		plan, err = s.PB.Update(ctx, "app_backend_plans", stringValue(plan["id"]), map[string]any{"status": "applying", "receipt": receipt})
		if err != nil {
			writeError(w, 503, "计划状态保存失败")
			return
		}
	}
	for _, operation := range operations {
		if completedIDs[stringValue(operation["id"])] {
			continue
		}
		candidate := byID[stringValue(operation["candidate_id"])]
		resolved, message := s.resolveApplyInput(ctx, app, candidate, asMap(operation["input"]), mapSliceAny(operations), completed)
		if message != "" {
			break
		}
		var result map[string]any
		err = s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
			var applyErr error
			result, applyErr = s.applyBackendPlanOperation(ctx, r, tx, app, candidate, resolved, stringValue(operation["id"]))
			if applyErr != nil {
				return applyErr
			}
			step := map[string]any{"operation_id": operation["id"], "candidate_id": candidate.ID, "capability": candidate.Capability, "result": result}
			newCompleted := append(append([]any{}, completed...), step)
			newIDs := cloneAnyMap(resourceIDs)
			newIDs[stringValue(operation["candidate_id"])] = result
			pending := []any{}
			seenCurrent := false
			for _, remaining := range operations {
				if stringValue(remaining["id"]) == stringValue(operation["id"]) {
					seenCurrent = true
					continue
				}
				if seenCurrent && !completedIDs[stringValue(remaining["id"])] {
					pending = append(pending, remaining)
				}
			}
			newReceipt := map[string]any{"status": "applying", "completed_steps": newCompleted, "remaining_steps": pending, "resource_ids": newIDs}
			_, updateErr := tx.Update(ctx, "app_backend_plans", stringValue(plan["id"]), map[string]any{"receipt": newReceipt})
			return updateErr
		})
		if err != nil {
			break
		}
		step := map[string]any{"operation_id": operation["id"], "candidate_id": candidate.ID, "capability": candidate.Capability, "result": result}
		completed = append(completed, step)
		completedIDs[stringValue(operation["id"])] = true
		resourceIDs[candidate.ID] = result
	}
	remaining := []any{}
	for _, operation := range operations {
		if !completedIDs[stringValue(operation["id"])] {
			remaining = append(remaining, operation)
		}
	}
	status := "applied"
	if len(remaining) > 0 {
		status = "partial"
	}
	finalReceipt := map[string]any{"status": status, "completed_steps": completed, "remaining_steps": remaining, "resource_ids": resourceIDs}
	plan, err = s.PB.Update(ctx, "app_backend_plans", stringValue(plan["id"]), map[string]any{"status": status, "revision": intValue(plan["revision"]) + 1, "receipt": finalReceipt})
	if err != nil {
		writeError(w, 503, "计划回执保存失败")
		return
	}
	writeJSON(w, 200, publicBackendPlan(plan))
}

func (s *Server) resolveApplyInput(ctx context.Context, app map[string]any, candidate backendPlanCandidate, input map[string]any, operations, completed []any) (map[string]any, string) {
	copy := cloneAnyMap(input)
	if candidate.Capability != "business_actions.create" && candidate.Capability != "workflows.configure" {
		return copy, ""
	}
	definition := asMap(copy["definition"])
	refKey := "table_ref"
	ref := stringValue(definition[refKey])
	if ref == "" {
		return nil, "缺少数据表候选引用"
	}
	if result := asMap(asMap(asMap(asMap(completedResourceMap(completed))[ref])["result"])); len(result) > 0 {
		definition["table"] = result["slug"]
		delete(definition, refKey)
		copy["definition"] = definition
		return copy, ""
	}
	resources, _, err := s.backendPlanSnapshot(ctx, app, stringValue(app["tenant_id"]))
	if err != nil {
		return nil, "应用数据表暂不可用"
	}
	table, ok := backendTableRef(ref, asSliceMap(resources["tables"]), stringValue(app["tenant_id"]), stringValue(app["id"]))
	if !ok {
		return nil, "数据表候选已失效"
	}
	definition["table"] = table["slug"]
	delete(definition, refKey)
	copy["definition"] = definition
	return copy, ""
}

func completedResourceMap(completed []any) map[string]any {
	out := map[string]any{}
	for _, value := range completed {
		step := asMap(value)
		out[stringValue(step["candidate_id"])] = step
	}
	return out
}

func (s *Server) applyBackendPlanOperation(ctx context.Context, r *http.Request, tx *pocketbase.Client, app map[string]any, candidate backendPlanCandidate, input map[string]any, operationID string) (map[string]any, error) {
	id := who(r)
	actor := id.actor(stringValue(app["id"]), "interactive")
	if candidate.Capability != "members.assign" {
		if _, err := s.authorizeWrite(ctx, tx, actor, false); err != nil {
			return nil, err
		}
	}
	switch candidate.Capability {
	case "collections.create":
		name, slug := strings.TrimSpace(stringValue(input["name"])), stringValue(input["slug"])
		pbName := "app_" + stringValue(app["id"]) + "_" + slug
		schemaFields := []map[string]any{{"name": "created", "type": "autodate", "onCreate": true, "system": true}, {"name": "updated", "type": "autodate", "onCreate": true, "onUpdate": true, "system": true}, {"name": "app_id", "type": "text", "required": true, "max": 64}, {"name": "tenant_id", "type": "text", "required": true, "max": 64}}
		for _, field := range asSliceMap(input["fields"]) {
			schemaFields = append(schemaFields, pbSchemaField(field))
		}
		if _, err := tx.CreateCollection(ctx, map[string]any{"type": "base", "name": pbName, "listRule": nil, "viewRule": nil, "createRule": nil, "updateRule": nil, "deleteRule": nil, "fields": schemaFields}); err != nil {
			return nil, err
		}
		meta, err := tx.Create(ctx, "app_collections", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "name": clip(name, 160), "slug": slug, "pb_collection": pbName, "fields": input["fields"]})
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": meta["id"], "slug": slug, "name": name, "operation_id": operationID}, nil
	case "collections.update":
		meta, err := tx.Get(ctx, "app_collections", stringValue(candidate.Bound["table_id"]))
		if err != nil || meta["tenant_id"] != id.Tenant["id"] || meta["app_id"] != app["id"] {
			return nil, errors.New("table reference is no longer valid")
		}
		old, fields := asSliceMap(meta["fields"]), asSliceMap(input["fields"])
		newNames := map[string]bool{}
		for _, field := range fields {
			newNames[stringValue(field["name"])] = true
		}
		removed := []map[string]any{}
		for _, field := range old {
			if !newNames[stringValue(field["name"])] {
				removed = append(removed, field)
			}
		}
		remove := stringSet(input["remove_fields"])
		for _, field := range removed {
			if !remove[stringValue(field["name"])] {
				return nil, errors.New("removed fields must be named explicitly")
			}
		}
		if len(removed) > 0 && input["confirm_data_loss"] != true {
			return nil, errors.New("field removal requires explicit data-loss confirmation")
		}
		if len(removed) > 0 && s.publishedUIUses(ctx, app, stringValue(meta["slug"]), removed) {
			return nil, errors.New("fields are used by the published UI")
		}
		if len(removed) > 0 {
			referenced, e := s.fieldHasConfiguredReferences(ctx, stringValue(id.Tenant["id"]), stringValue(app["id"]), removed)
			if e != nil || referenced {
				return nil, errors.New("fields are still referenced by configuration")
			}
		}
		schema, err := tx.Collection(ctx, stringValue(meta["pb_collection"]))
		if err != nil {
			return nil, err
		}
		_, count, _, err := tx.List(ctx, stringValue(meta["pb_collection"]), listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "", 1, 1)
		if err != nil {
			return nil, err
		}
		schemaFields := []map[string]any{}
		for _, stored := range asSliceMap(schema["fields"]) {
			if reservedAppFields[stringValue(stored["name"])] {
				schemaFields = append(schemaFields, stored)
			}
		}
		for _, field := range fields {
			if boolValue(field["required"]) && findField(old, stringValue(field["name"])) == nil && count > 0 {
				return nil, errors.New("cannot add a required field to a nonempty table")
			}
			var existing map[string]any
			for _, stored := range asSliceMap(schema["fields"]) {
				if stored["name"] == field["name"] {
					existing = stored
					break
				}
			}
			if existing == nil {
				schemaFields = append(schemaFields, pbSchemaField(field))
			} else {
				existing["required"] = field["required"]
				if field["type"] == "select" {
					existing["values"] = field["options"]
				}
				schemaFields = append(schemaFields, existing)
			}
		}
		newSchema := map[string]any{"type": "base", "name": schema["name"], "listRule": nil, "viewRule": nil, "createRule": nil, "updateRule": nil, "deleteRule": nil, "fields": schemaFields, "indexes": schema["indexes"]}
		if _, err := tx.UpdateCollection(ctx, stringValue(meta["pb_collection"]), newSchema); err != nil {
			return nil, err
		}
		updated, err := tx.Update(ctx, "app_collections", stringValue(meta["id"]), map[string]any{"fields": fields})
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": updated["id"], "slug": updated["slug"], "name": updated["name"], "operation_id": operationID}, nil
	case "business_actions.create":
		definition, message := s.normalizeBusinessAction(ctx, app, input["definition"])
		if message != "" {
			return nil, errors.New(message)
		}
		saved, err := tx.Create(ctx, "business_actions", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "created_by": id.User["id"], "name": clip(strings.TrimSpace(stringValue(input["name"])), 160), "description": clip(stringValue(input["description"]), 1000), "definition": definition, "status": "draft", "revision": 1})
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": saved["id"], "name": saved["name"], "status": saved["status"], "operation_id": operationID}, nil
	case "workflows.configure":
		definition, message := s.normalizeWorkflow(ctx, app, input["definition"])
		if message != "" {
			return nil, errors.New(message)
		}
		saved, err := tx.Create(ctx, "workflows", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "created_by": id.User["id"], "name": clip(strings.TrimSpace(stringValue(input["name"])), 160), "description": clip(stringValue(input["description"]), 1000), "definition": definition, "status": "draft", "revision": 1})
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": saved["id"], "name": saved["name"], "status": saved["status"], "operation_id": operationID}, nil
	case "members.assign":
		tenant, err := tx.Get(ctx, "tenants", stringValue(id.Tenant["id"]))
		if err != nil || tenant["owner_id"] != id.User["id"] {
			return nil, businessError(403, "only the workspace owner may assign app access")
		}
		userID := stringValue(candidate.Bound["user_id"])
		member, err := tx.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "user_id = "+pbFilterString(userID)))
		if err != nil || member["role"] == "owner" {
			return nil, errors.New("permission member is no longer valid")
		}
		old, findErr := tx.Find(ctx, "app_members", listFilter("tenant_id = "+pbFilterString(stringValue(app["tenant_id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "user_id = "+pbFilterString(userID)))
		if stringValue(input["role"]) == "remove" {
			if findErr == nil {
				if err := tx.Delete(ctx, "app_members", stringValue(old["id"])); err != nil {
					return nil, err
				}
			}
			return map[string]any{"user_id": userID, "role": "removed", "operation_id": operationID}, nil
		}
		values := map[string]any{"tenant_id": app["tenant_id"], "app_id": app["id"], "user_id": userID, "role": input["role"], "can_batch": boolValue(input["can_batch"])}
		var saved map[string]any
		if findErr == nil {
			saved, err = tx.Update(ctx, "app_members", stringValue(old["id"]), map[string]any{"role": values["role"], "can_batch": values["can_batch"]})
		} else {
			saved, err = tx.Create(ctx, "app_members", values)
		}
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": saved["id"], "user_id": userID, "role": saved["role"], "operation_id": operationID}, nil
	}
	return nil, fmt.Errorf("unsupported backend plan capability %q", candidate.Capability)
}

func backendHarnessCandidates(ctx context.Context, pb *pocketbase.Client, tenantID, appID, actorID string) ([]harness.CandidateOption, error) {
	app, err := pb.Get(ctx, "apps", appID)
	if err != nil || app["tenant_id"] != tenantID || boolValue(app["archived"]) {
		return nil, harness.ErrChooserUnavailable
	}
	user, err := pb.Get(ctx, "users", actorID)
	if err != nil || boolValue(user["disabled"]) {
		return nil, harness.ErrChooserUnavailable
	}
	tenant, err := pb.Get(ctx, "tenants", tenantID)
	if err != nil {
		return nil, harness.ErrChooserUnavailable
	}
	membership, err := pb.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(tenantID), "user_id = "+pbFilterString(actorID)))
	if err != nil {
		return nil, harness.ErrChooserUnavailable
	}
	access, err := applicationAccess(ctx, pb, app, identity{User: user, Tenant: tenant, Membership: membership})
	if err != nil || access.Role == "" {
		return nil, harness.ErrChooserUnavailable
	}
	options := []harness.CandidateOption{}
	add := func(capability, description string, input map[string]any, write bool, parts ...string) {
		options = append(options, harness.CandidateOption{ID: backendOpaqueID("bp1-", tenantID+"\x00"+appID, parts...), Capability: capability, Description: description, Input: input, Write: write})
	}
	tables, err := pb.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(appID)), "created")
	if err != nil {
		return nil, harness.ErrChooserUnavailable
	}
	for _, table := range tables {
		add("records.query", "Query "+stringValue(table["name"]), map[string]any{"table": stringValue(table["slug"]), "page": 1}, false, "records.query", stringValue(table["id"]))
	}
	if canManageAppRole(string(access.Role)) {
		for _, table := range tables {
			slug := stringValue(table["slug"])
			fields := []any{}
			for _, field := range asSliceMap(table["fields"]) {
				name := stringValue(field["name"])
				if name != "" && stringValue(field["type"]) != "file" && stringValue(field["type"]) != "relation" {
					fields = append(fields, name)
				}
			}
			if len(fields) == 0 {
				continue
			}
			sourceID := slug + "_source"
			definition := map[string]any{"schema_version": 3, "title": stringValue(table["name"]), "pages": []any{map[string]any{"id": slug, "title": stringValue(table["name"]), "data_sources": []any{map[string]any{"id": sourceID, "collection": slug, "fields": fields, "actions": []any{}}}, "spec": map[string]any{"root": "page", "elements": map[string]any{"page": map[string]any{"type": "Page", "props": map[string]any{"title": stringValue(table["name"])}, "children": []any{"table"}}, "table": map[string]any{"type": "RecordTable", "props": map[string]any{"source": sourceID, "title": stringValue(table["name"])}, "children": []any{}}}}}}}
			add("ui.compose", "Compose a json-render page for "+stringValue(table["name"]), map[string]any{"definition": definition, "table_id": table["id"]}, true, "ui.compose", stringValue(table["id"]))
		}
	}
	if canManageAppRole(string(access.Role)) {
		plans, planErr := pb.ListAll(ctx, "app_backend_plans", listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(appID), "user_id = "+pbFilterString(actorID), "(status = \"draft\" || status = \"applying\")"), "created")
		if planErr != nil {
			return nil, harness.ErrChooserUnavailable
		}
		for _, plan := range plans {
			add("backend_plan.apply", "Apply confirmed BackendPlan "+stringValue(plan["id"]), map[string]any{"plan_id": stringValue(plan["id"]), "expected_revision": intValue(plan["revision"])}, true, "backend_plan.apply", stringValue(plan["id"]))
		}
	}
	if access.Role.canWrite() {
		actions, listErr := optionalBackendRows(ctx, pb, "business_actions", listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(appID), "status = \"enabled\""), "created")
		if listErr != nil {
			return nil, harness.ErrChooserUnavailable
		}
		for _, action := range actions {
			add("business_actions.execute", "Execute "+stringValue(action["name"]), map[string]any{"action_id": stringValue(action["id"]), "input": map[string]any{}}, true, "business_actions.execute", stringValue(action["id"]))
		}
	}
	return options, nil
}
