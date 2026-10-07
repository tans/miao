package httpapi

import (
	"net/http"

	"github.com/tans/miao/internal/api"
)

// openAPIDocument serves the generated API contract to documentation tools and
// clients. It is intentionally public so a deployment can be inspected before
// authentication without exposing business data.
func (s *Server) openAPIDocument(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.OpenAPI())
}

// apiSchema serves the DTO component schemas used by the OpenAPI document.
func (s *Server) apiSchema(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$id":     "https://miao.local/schema/api.json",
		"title":   "MIAO API schemas",
		"$defs":   api.Schemas(),
	})
}

// configSchema serves the settings schema derived from internal/settings DTOs.
func (s *Server) configSchema(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.ConfigSchema())
}
