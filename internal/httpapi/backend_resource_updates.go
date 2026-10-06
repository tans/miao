package httpapi

import (
	"context"
	"strings"

	"github.com/tans/miao/internal/pocketbase"
)

func (s *Server) normalizePlanResourceBaseline(ctx context.Context, app map[string]any, candidate backendPlanCandidate, input map[string]any, collection string) string {
	resource, err := s.PB.Get(ctx, collection, stringValue(candidate.Bound["resource_id"]))
	if err != nil || resource["app_id"] != app["id"] || resource["tenant_id"] != app["tenant_id"] || resource["status"] == "archived" {
		return "业务配置候选已失效"
	}
	if intValue(input["expected_revision"]) != intValue(resource["revision"]) {
		return "业务配置修订已变化，请重新读取"
	}
	input["resource_id"] = resource["id"]
	input["baseline_status"], input["baseline_definition"] = resource["status"], resource["definition"]
	return ""
}

func applyPlanResourceUpdate(ctx context.Context, tx *pocketbase.Client, app map[string]any, candidate backendPlanCandidate, input, definition map[string]any, collection, operationID string) (map[string]any, error) {
	resource, err := tx.Get(ctx, collection, stringValue(candidate.Bound["resource_id"]))
	if err != nil {
		return nil, err
	}
	if resource["id"] != input["resource_id"] || resource["tenant_id"] != app["tenant_id"] || resource["app_id"] != app["id"] || resource["status"] == "archived" {
		return nil, businessError(409, "业务配置引用已失效")
	}
	if intValue(resource["revision"]) != intValue(input["expected_revision"]) || resource["status"] != input["baseline_status"] || !equalJSON(resource["definition"], input["baseline_definition"]) {
		return nil, businessError(409, "已审阅业务配置的修订、定义或启用状态已变化")
	}
	saved, err := tx.Update(ctx, collection, stringValue(resource["id"]), map[string]any{"name": clip(strings.TrimSpace(stringValue(input["name"])), 160), "description": clip(stringValue(input["description"]), 1000), "definition": definition, "revision": intValue(resource["revision"]) + 1, "status": "draft", "pause_reason": ""})
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": saved["id"], "name": saved["name"], "status": saved["status"], "revision": saved["revision"], "operation_id": operationID}, nil
}
