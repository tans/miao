package httpapi

func (s *Server) routesAdmin() {
	// adminAuth is the shared admin boundary in middleware.go; every
	// /api/admin route must go through it.
	admin := s.adminAuth
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
