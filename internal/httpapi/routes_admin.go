package httpapi

import "net/http"

func (s *Server) routesAdmin() {
	admin := func(next http.HandlerFunc) http.HandlerFunc {
		return s.auth(func(w http.ResponseWriter, r *http.Request) {
			id := who(r)
			ctx, cancel := contextTimeout(r)
			defer cancel()
			if !s.isAdmin(ctx, stringValue(id.User["email"])) {
				s.writeAdminAudit(r.Context(), id, "access.denied", "endpoint", clip(r.URL.Path, 64), "", 403)
				writeError(w, 403, "无权访问平台管理功能")
				return
			}
			next(w, r)
		})
	}
	s.Mux.HandleFunc("GET /api/admin/overview", admin(s.adminOverview))
	s.Mux.HandleFunc("GET /api/admin/runtime", admin(s.adminRuntime))
	s.Mux.HandleFunc("GET /api/admin/settings", admin(s.adminSettings))
	s.Mux.HandleFunc("PUT /api/admin/settings/registration", admin(s.adminRegistrationUpdate))
	s.Mux.HandleFunc("PUT /api/admin/settings/mail", admin(s.adminMailUpdate))
	s.Mux.HandleFunc("PUT /api/admin/settings/admins", admin(s.adminAdminsUpdate))
	s.Mux.HandleFunc("PUT /api/admin/settings/backup", admin(s.adminBackupUpdate))
	s.Mux.HandleFunc("PUT /api/admin/settings/branding", admin(s.adminBrandingUpdate))
	s.Mux.HandleFunc("GET /api/admin/users", admin(s.adminUsers))
	s.Mux.HandleFunc("PATCH /api/admin/users/{id}/status", admin(s.adminUserStatus))
	s.Mux.HandleFunc("GET /api/admin/workspaces", admin(s.adminWorkspaces))
	s.Mux.HandleFunc("GET /api/admin/apps", admin(s.adminApps))
	s.Mux.HandleFunc("GET /api/admin/usage", admin(s.adminUsage))
	s.Mux.HandleFunc("GET /api/admin/audit", admin(s.adminAudit))
	s.Mux.HandleFunc("GET /api/admin/ai", admin(s.adminAIServices))
	s.Mux.HandleFunc("PUT /api/admin/ai/{kind}", admin(s.adminAIServiceUpdate))
	s.Mux.HandleFunc("DELETE /api/admin/ai/{kind}", admin(s.adminAIServiceReset))
	s.Mux.HandleFunc("POST /api/admin/ai/{kind}/check", admin(s.adminAIServiceCheck))
	s.Mux.HandleFunc("GET /api/admin/usage/requests", admin(s.adminUsageRequests))
}
