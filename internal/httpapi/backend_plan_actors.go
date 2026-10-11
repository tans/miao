package httpapi

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tans/miao/internal/harness"
	"github.com/tans/miao/internal/pocketbase"
)

// Backend plan operations driven by the agent/harness flow instead of HTTP
// handlers. The /api/apps/{id}/backend endpoints stay in backend_plans.go.

func (s *Server) createBackendPlanForActor(ctx context.Context, actor executionActor, requestedOperations []backendPlanInput, stepID string) (map[string]any, error) {
	id, app, access, err := s.buildActor(ctx, s.PB, actor)
	if err != nil {
		return nil, err
	}
	role := string(access)
	if len(requestedOperations) == 0 || len(requestedOperations) > backendPlanMaxOperations {
		return nil, businessError(400, "计划需要 1–20 个操作")
	}
	if stepID != "" {
		prior, err := s.PB.Find(ctx, "app_backend_plans", "harness_step_id = "+pbFilterString(stepID))
		if err == nil {
			if prior["app_id"] != actor.AppID || prior["tenant_id"] != actor.TenantID || prior["user_id"] != actor.UserID {
				return nil, businessError(403, "计划回执范围不匹配")
			}
			return prior, nil
		}
		if !isMissing(err) {
			return nil, err
		}
	}
	candidates, _, _, err := s.backendPlanCandidates(ctx, app, role, id)
	if err != nil {
		return nil, businessError(503, "后端候选暂不可用")
	}
	byID := backendCandidateIndex(candidates)
	resources, baseline, err := s.backendPlanSnapshot(ctx, app, stringValue(id.Tenant["id"]))
	if err != nil {
		return nil, businessError(503, "应用基线暂不可用")
	}
	tables, operations := asSliceMap(resources["tables"]), make([]backendPlanOperation, 0, len(requestedOperations))
	impactSet, seenCandidates := map[string]bool{}, map[string]bool{}
	tableCandidateID := ""
	for index, requested := range requestedOperations {
		candidate, ok := byID[requested.CandidateID]
		if !ok || seenCandidates[candidate.ID] {
			return nil, businessError(400, "候选不存在、重复或不可用")
		}
		seenCandidates[candidate.ID] = true
		if candidate.Capability == "collections.create" {
			tableCandidateID = candidate.ID
		}
		input, dependencies, message := s.normalizePlanOperation(ctx, app, candidate, requested.Input, tables, index+1, tableCandidateID)
		if message != "" {
			return nil, businessError(400, message)
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
	row, err := s.PB.Create(ctx, "app_backend_plans", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "user_id": id.User["id"], "status": "draft", "revision": 1, "baseline_hash": baseline, "expires_at": expires, "operations": operationValues, "dependency_order": order, "impact": impactValues, "receipt": initialReceipt, "harness_step_id": stepID})
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Server) applyBackendPlanForActor(ctx context.Context, actor executionActor, planID string, expectedRevision int) (map[string]any, error) {
	lock := sessionLock("backend-plan:" + planID)
	lock.Lock()
	defer lock.Unlock()
	id, app, access, err := s.buildActor(ctx, s.PB, actor)
	if err != nil {
		return nil, err
	}
	role := string(access)
	plan, err := s.PB.Get(ctx, "app_backend_plans", planID)
	if err != nil || plan["app_id"] != actor.AppID || plan["tenant_id"] != actor.TenantID || plan["user_id"] != actor.UserID {
		return nil, businessError(404, "计划不存在")
	}
	if plan["status"] == "applied" || plan["status"] == "partial" {
		return plan, nil
	}
	if expectedRevision != intValue(plan["revision"]) {
		return nil, businessError(409, "计划版本已变化，请重新读取")
	}
	if !parseTime(plan["expires_at"]).After(time.Now()) {
		return nil, businessError(409, "计划已过期，请重新创建")
	}
	if plan["status"] != "draft" && plan["status"] != "applying" {
		return nil, businessError(409, "计划当前状态不能应用")
	}
	_, baseline, err := s.backendPlanSnapshot(ctx, app, stringValue(id.Tenant["id"]))
	if err != nil {
		return nil, businessError(503, "应用基线暂不可用")
	}
	if plan["status"] == "draft" && baseline != stringValue(plan["baseline_hash"]) {
		return nil, businessError(409, "应用已变化，请重新读取并创建计划")
	}
	candidates, _, _, err := s.backendPlanCandidates(ctx, app, role, id)
	if err != nil {
		return nil, businessError(503, "后端候选暂不可用")
	}
	byID := backendCandidateIndex(candidates)
	operations := asSliceMap(plan["operations"])
	for _, operation := range operations {
		if _, ok := byID[stringValue(operation["candidate_id"])]; !ok {
			return nil, businessError(409, "计划候选或权限已失效")
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
			return nil, businessError(503, "计划状态保存失败")
		}
	}
	for _, operation := range operations {
		if completedIDs[stringValue(operation["id"])] {
			continue
		}
		candidate := byID[stringValue(operation["candidate_id"])]
		resolved, message := s.resolveApplyInput(ctx, app, candidate, asMap(operation["input"]), mapSliceAny(operations), completed)
		if message != "" {
			err = businessError(409, message)
			break
		}
		var result map[string]any
		err = s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
			var applyErr error
			result, applyErr = s.applyBackendPlanOperation(ctx, id, tx, app, candidate, resolved, stringValue(operation["id"]))
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
	if err != nil {
		finalReceipt["error"] = err.Error()
	}
	plan, err = s.PB.Update(ctx, "app_backend_plans", stringValue(plan["id"]), map[string]any{"status": status, "revision": intValue(plan["revision"]) + 1, "receipt": finalReceipt})
	if err != nil {
		return nil, businessError(503, "计划回执保存失败")
	}
	return plan, nil
}

func (s *Server) applyBackendPlanOperation(ctx context.Context, id identity, tx *pocketbase.Client, app map[string]any, candidate backendPlanCandidate, input map[string]any, operationID string) (map[string]any, error) {
	actor := id.actor(stringValue(app["id"]), "interactive")
	current, _, _, err := s.buildActor(ctx, tx, actor)
	if err != nil {
		return nil, err
	}
	id = current
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
		updates := map[string]any{"fields": fields}
		if name, supplied := input["name"]; supplied {
			updates["name"] = name
		}
		updated, err := tx.Update(ctx, "app_collections", stringValue(meta["id"]), updates)
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": updated["id"], "slug": updated["slug"], "name": updated["name"], "operation_id": operationID}, nil
	case "business_actions.create", "business_actions.update":
		tables, err := tx.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "")
		if err != nil {
			return nil, err
		}
		definition, message := normalizeBusinessActionForTables(input["definition"], tables)
		if message != "" {
			return nil, errors.New(message)
		}
		if candidate.Capability == "business_actions.update" {
			return applyPlanResourceUpdate(ctx, tx, app, candidate, input, definition, "business_actions", operationID)
		}
		saved, err := tx.Create(ctx, "business_actions", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "created_by": id.User["id"], "name": clip(strings.TrimSpace(stringValue(input["name"])), 160), "description": clip(stringValue(input["description"]), 1000), "definition": definition, "status": "draft", "revision": 1})
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": saved["id"], "name": saved["name"], "status": saved["status"], "operation_id": operationID}, nil
	case "connectors.configure":
		if strings.TrimSpace(stringValue(input["name"])) == "" {
			return nil, businessError(400, "请输入连接器名称")
		}
		definition, message := normalizeConnectorDefinition(input["definition"])
		if message != "" {
			return nil, businessError(400, message)
		}
		saved, err := tx.Create(ctx, "connectors", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "created_by": id.User["id"], "name": clip(strings.TrimSpace(stringValue(input["name"])), 160), "description": clip(stringValue(input["description"]), 1000), "definition": definition, "status": "draft", "revision": 1, "pause_reason": ""})
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": saved["id"], "name": saved["name"], "status": saved["status"], "revision": saved["revision"], "operation_id": operationID}, nil
	case "collection_scripts.configure":
		if _, _, err := s.collectionScriptMutationActor(ctx, tx, actor, nil); err != nil {
			return nil, err
		}
		if strings.TrimSpace(stringValue(input["name"])) == "" {
			return nil, businessError(400, "请输入采集脚本名称")
		}
		definition := asMap(input["definition"])
		definition, message := resolveCollectionScriptPlanDefinition(ctx, tx, app, definition)
		if message != "" {
			return nil, businessError(409, message)
		}
		normalized, message := normalizeCollectionScriptDefinitionWithConnector(ctx, tx, stringValue(id.Tenant["id"]), stringValue(app["id"]), definition, true)
		if message != "" {
			return nil, businessError(400, message)
		}
		saved, err := tx.Create(ctx, "collection_scripts", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "created_by": id.User["id"], "name": clip(strings.TrimSpace(stringValue(input["name"])), 160), "revision": 1, "definition": normalized, "status": "draft", "pause_reason": "", "next_run_at": ""})
		if err != nil {
			return nil, err
		}
		if _, err = tx.Create(ctx, "collection_script_versions", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "script_id": saved["id"], "version": 1, "created_by": id.User["id"], "definition": normalized}); err != nil {
			return nil, err
		}
		return map[string]any{"id": saved["id"], "name": saved["name"], "status": saved["status"], "revision": saved["revision"], "operation_id": operationID}, nil
	case "workflows.configure", "workflows.update":
		tables, err := tx.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "")
		if err != nil {
			return nil, err
		}
		definition, message := normalizeWorkflowForTables(input["definition"], tables)
		if message != "" {
			return nil, errors.New(message)
		}
		if candidate.Capability == "workflows.update" {
			return applyPlanResourceUpdate(ctx, tx, app, candidate, input, definition, "workflows", operationID)
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
		latest, _, _, err := pb.List(ctx, "app_versions", listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(appID)), "-version", 1, 1)
		if err != nil {
			return nil, err
		}
		baseID := ""
		if len(latest) > 0 {
			baseID = stringValue(latest[0]["id"])
		}
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
			add("ui.compose", "Compose a json-render page for "+stringValue(table["name"]), map[string]any{"definition": definition, "table_id": table["id"], "based_on_version_id": baseID, "expected_latest_version_id": baseID}, true, "ui.compose", stringValue(table["id"]))
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
