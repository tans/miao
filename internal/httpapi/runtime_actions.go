package httpapi

import (
	"context"
	"encoding/json"

	"github.com/tans/miao/internal/pocketbase"
)

func workflowTransition(definition map[string]any, id string) map[string]any {
	for _, transition := range asSliceMap(definition["transitions"]) {
		if transition["id"] == id {
			return transition
		}
	}
	return nil
}

func runtimeActionKey(id identity, version string, action, input map[string]any) string {
	data, _ := json.Marshal(map[string]any{"user": id.User["id"], "version": version, "action": action, "page": input["ui_page"], "record": input["record_id"], "updated": input["expected_updated_at"], "input": input["input"], "request": input["idempotency_key"]})
	return backendOpaqueID("ui-", stringValue(id.Tenant["id"]), "action", string(data))
}

func (s *Server) runtimeActionGuard(ctx context.Context, id identity, app, source map[string]any, tables []map[string]any, version, recordID, updated string) func(*pocketbase.Client) error {
	return func(tx *pocketbase.Client) error {
		fresh, err := tx.Get(ctx, "apps", stringValue(app["id"]))
		if err != nil {
			return err
		}
		if fresh["published_version_id"] != version || boolValue(fresh["archived"]) {
			return businessError(409, "正式界面已变化，请刷新后重试")
		}
		for _, table := range tables {
			if table["slug"] != source["collection"] {
				continue
			}
			current, err := tx.Get(ctx, "app_collections", stringValue(table["id"]))
			if err != nil {
				return err
			}
			if current["tenant_id"] != id.Tenant["id"] || current["app_id"] != app["id"] || !equalJSON(current["fields"], table["fields"]) {
				return businessError(409, "动作数据表定义已变化，请重新读取")
			}
			row, err := tx.Get(ctx, stringValue(current["pb_collection"]), recordID)
			if err != nil || row["tenant_id"] != id.Tenant["id"] || row["app_id"] != app["id"] {
				return businessError(404, "动作目标记录不存在")
			}
			if stringValue(row["updated"]) != updated {
				return businessError(409, "记录已变化，请刷新后重试")
			}
			return nil
		}
		return businessError(404, "动作数据绑定不存在")
	}
}

func (s *Server) runtimeUIActions(ctx context.Context, app, source map[string]any) ([]map[string]any, error) {
	actions := []map[string]any{}
	for _, stored := range asSliceMap(source["actions"]) {
		action := cloneAnyMap(stored)
		action["available"] = true
		if id := stringValue(action["action_id"]); id != "" {
			row, err := s.PB.Get(ctx, "business_actions", id)
			if err != nil && !isMissing(err) {
				return nil, err
			}
			action["available"] = err == nil && row["app_id"] == app["id"] && row["tenant_id"] == app["tenant_id"] && row["status"] == "enabled" && intValue(action["action_revision"]) == intValue(row["revision"])
			if boolValue(action["available"]) {
				inputs := []map[string]any{}
				for _, input := range asSliceMap(asMap(row["definition"])["inputs"]) {
					if input["name"] != "record_id" && input["name"] != "record_updated_at" {
						inputs = append(inputs, input)
					}
				}
				action["inputs"] = inputs
			}
		}
		if id := stringValue(action["workflow_id"]); id != "" {
			row, err := s.PB.Get(ctx, "workflows", id)
			if err != nil && !isMissing(err) {
				return nil, err
			}
			definition := asMap(row["definition"])
			transition := workflowTransition(definition, stringValue(action["transition_id"]))
			action["available"] = err == nil && row["app_id"] == app["id"] && row["tenant_id"] == app["tenant_id"] && row["status"] == "enabled" && intValue(action["workflow_revision"]) == intValue(row["revision"]) && definition["table"] == source["collection"] && transition != nil
			if boolValue(action["available"]) {
				action["state_field"], action["from"] = definition["state_field"], transition["from"]
			}
		}
		actions = append(actions, action)
	}
	return actions, nil
}
