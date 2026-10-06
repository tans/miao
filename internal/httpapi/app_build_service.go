package httpapi

import (
	"context"
	"strings"

	"github.com/tans/miao/internal/pocketbase"
)

func (s *Server) workspaceActor(ctx context.Context, pb *pocketbase.Client, actor executionActor) (identity, error) {
	user, err := pb.Get(ctx, "users", actor.UserID)
	if err != nil {
		return identity{}, err
	}
	if boolValue(user["disabled"]) || s.RequireVerification && !boolValue(user["verified"]) {
		return identity{}, businessError(403, "账号当前不能执行此操作")
	}
	tenant, err := pb.Get(ctx, "tenants", actor.TenantID)
	if err != nil {
		return identity{}, err
	}
	member, err := pb.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(actor.TenantID), "user_id = "+pbFilterString(actor.UserID)))
	if err != nil {
		if isMissing(err) {
			return identity{}, businessError(403, "你已不再是此工作区成员")
		}
		return identity{}, err
	}
	return identity{User: user, Tenant: tenant, Membership: member}, nil
}

func (s *Server) buildActor(ctx context.Context, pb *pocketbase.Client, actor executionActor) (identity, map[string]any, appRole, error) {
	id, err := s.workspaceActor(ctx, pb, actor)
	if err != nil {
		return id, nil, "", err
	}
	app, err := pb.Get(ctx, "apps", actor.AppID)
	if err != nil {
		return id, nil, "", err
	}
	access, err := applicationAccess(ctx, pb, app, id)
	if err != nil {
		return id, nil, "", err
	}
	if boolValue(app["archived"]) || !access.Role.canManage() {
		return id, nil, "", businessError(403, "你没有管理此应用的权限")
	}
	return id, app, access.Role, nil
}

func (s *Server) createApplication(ctx context.Context, actor executionActor, input map[string]any, stepID string) (map[string]any, error) {
	name := strings.TrimSpace(stringValue(input["name"]))
	if name == "" || len([]rune(name)) > 160 {
		return nil, businessError(400, "应用名称需要 1–160 个字符")
	}
	var app map[string]any
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		id, err := s.workspaceActor(ctx, tx, actor)
		if err != nil {
			return err
		}
		role := appRole("publisher")
		if id.Membership["role"] == "owner" && id.Tenant["owner_id"] == id.User["id"] {
			role = "owner"
		}
		if stepID != "" {
			prior, err := tx.Find(ctx, "apps", "harness_step_id = "+pbFilterString(stepID))
			if err == nil {
				if prior["tenant_id"] != actor.TenantID || prior["creator_id"] != actor.UserID || boolValue(prior["archived"]) {
					return businessError(409, "创建应用的回执已失效")
				}
				app = prior
				app["permission"] = string(role)
				return nil
			}
			if !isMissing(err) {
				return err
			}
		}
		app, err = tx.Create(ctx, "apps", map[string]any{"tenant_id": actor.TenantID, "creator_id": actor.UserID, "name": name, "description": clip(stringValue(input["description"]), 4000), "harness_step_id": stepID})
		if err != nil {
			return err
		}
		if role == "publisher" {
			if _, err := tx.Create(ctx, "app_members", map[string]any{"tenant_id": actor.TenantID, "app_id": app["id"], "user_id": actor.UserID, "role": role, "can_batch": true}); err != nil {
				return err
			}
		}
		app["permission"] = string(role)
		return nil
	})
	return app, err
}
