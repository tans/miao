package httpapi

import (
	"context"

	"github.com/tans/miao/internal/pocketbase"
)

func (s *Server) createBusinessConfiguration(ctx context.Context, id identity, app map[string]any, collection string, data map[string]any) (map[string]any, error) {
	var saved map[string]any
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		if _, _, _, err := s.buildActor(ctx, tx, id.actor(stringValue(app["id"]), "interactive")); err != nil {
			return err
		}
		if err := validateBusinessConfiguration(ctx, tx, app, collection, data); err != nil {
			return err
		}
		var err error
		saved, err = tx.Create(ctx, collection, data)
		return err
	})
	return saved, err
}

func (s *Server) updateBusinessConfiguration(ctx context.Context, id identity, app, baseline map[string]any, collection string, updates map[string]any) (map[string]any, error) {
	var saved map[string]any
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		if _, _, _, err := s.buildActor(ctx, tx, id.actor(stringValue(app["id"]), "interactive")); err != nil {
			return err
		}
		current, err := tx.Get(ctx, collection, stringValue(baseline["id"]))
		if err != nil {
			return err
		}
		if current["tenant_id"] != id.Tenant["id"] || current["app_id"] != app["id"] {
			return businessError(404, "业务配置不存在")
		}
		if intValue(current["revision"]) != intValue(baseline["revision"]) || current["status"] != baseline["status"] || !equalJSON(current["definition"], baseline["definition"]) {
			return businessError(409, "业务配置或启用状态已变化，请重新载入")
		}
		definition := current["definition"]
		if updates["definition"] != nil {
			definition = updates["definition"]
		}
		if updates["status"] != "paused" {
			if err := validateBusinessConfiguration(ctx, tx, app, collection, map[string]any{"definition": definition}); err != nil {
				return err
			}
		}
		saved, err = tx.Update(ctx, collection, stringValue(baseline["id"]), updates)
		return err
	})
	return saved, err
}

func validateBusinessConfiguration(ctx context.Context, tx *pocketbase.Client, app map[string]any, collection string, data map[string]any) error {
	tables, err := tx.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(app["tenant_id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "")
	if err != nil {
		return err
	}
	var definition map[string]any
	var message string
	switch collection {
	case "business_actions":
		definition, message = normalizeBusinessActionForTables(data["definition"], tables)
	case "workflows":
		definition, message = normalizeWorkflowForTables(data["definition"], tables)
	default:
		return businessError(400, "不支持的业务配置类型")
	}
	if message != "" {
		return businessError(409, "业务配置引用需要重新审阅："+message)
	}
	data["definition"] = definition
	return nil
}
