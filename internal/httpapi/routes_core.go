package httpapi

import "net/http"

func (s *Server) registerCoreRoutes() {
	s.Mux.HandleFunc("GET /api/health", s.health)
	s.Mux.HandleFunc("GET /api/openapi.json", s.openAPIDocument)
	s.Mux.HandleFunc("GET /api/schema.json", s.apiSchema)
	s.Mux.HandleFunc("GET /api/config-schema.json", s.configSchema)
	s.Mux.HandleFunc("GET /api/build/templates", s.auth(s.listBuildTemplates))
}

func (s *Server) registerFallbackRoutes() {
	s.Mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { writeError(w, 404, "接口不存在") })
}
