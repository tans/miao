package httpapi

import (
	"context"
	"net/http"
)

// Shared application and workspace authorization helpers used across route
// domains. Domain route files must express permissions through these instead
// of duplicating checks.

func (s *Server) appPermission(ctx context.Context, app map[string]any, id identity) string {
	access, err := applicationAccess(ctx, s.PB, app, id)
	if err != nil {
		s.Logger.Error("app permission lookup failed", "error", err)
		return ""
	}
	return string(access.Role)
}

func (s *Server) appForRequest(ctx context.Context, r *http.Request) (map[string]any, string, error) {
	id := who(r)
	app, err := s.PB.Get(ctx, "apps", pathID(r, "id"))
	if err != nil {
		return nil, "", err
	}
	if app["tenant_id"] != id.Tenant["id"] {
		return nil, "", context.Canceled
	}
	role := s.appPermission(ctx, app, id)
	if role == "" {
		return nil, "", context.Canceled
	}
	app["permission"] = role
	return app, role, nil
}

func canPublishAppRole(role string) bool { return appRole(role).canPublish() }
func canManageAppRole(role string) bool  { return appRole(role).canManage() }
func (s *Server) appCanBatch(ctx context.Context, app map[string]any, id identity) bool {
	access, err := applicationAccess(ctx, s.PB, app, id)
	if err != nil {
		s.Logger.Error("app batch permission lookup failed", "error", err)
		return false
	}
	return access.Batch
}

func (s *Server) appTables(ctx context.Context, app map[string]any, tenantID string) ([]map[string]any, error) {
	if tenantID == "" {
		tenantID = stringValue(app["tenant_id"])
	}
	return s.PB.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(stringValue(app["id"]))), "created")
}

func (s *Server) tableForRequest(ctx context.Context, r *http.Request, app map[string]any) (map[string]any, error) {
	return s.PB.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "slug = "+pbFilterString(pathID(r, "slug"))))
}

func (s *Server) requireOwner(w http.ResponseWriter, id identity) bool {
	if id.Membership["role"] != "owner" || id.Tenant["owner_id"] != id.User["id"] {
		writeError(w, 403, "只有工作区所有者可以管理此设置")
		return false
	}
	return true
}
