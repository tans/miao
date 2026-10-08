package httpapi

func (s *Server) routesApps() {
	s.Mux.HandleFunc("GET /api/apps", s.auth(s.listApps))
	s.Mux.HandleFunc("POST /api/apps", s.auth(s.createApp))
	s.Mux.HandleFunc("GET /api/apps/{id}", s.auth(s.getApp))
	s.Mux.HandleFunc("PATCH /api/apps/{id}", s.auth(s.updateApp))
	s.Mux.HandleFunc("DELETE /api/apps/{id}", s.auth(s.deleteApp))
	s.Mux.HandleFunc("GET /api/apps/{id}/collections", s.auth(s.listTables))
	s.Mux.HandleFunc("POST /api/apps/{id}/collections", s.auth(s.createTable))
	s.Mux.HandleFunc("PATCH /api/apps/{id}/collections/{slug}", s.auth(s.updateTable))
	s.Mux.HandleFunc("DELETE /api/apps/{id}/collections/{slug}", s.auth(s.deleteTable))
	s.Mux.HandleFunc("GET /api/apps/{id}/collections/{slug}/records", s.auth(s.listRecords))
	s.Mux.HandleFunc("POST /api/apps/{id}/collections/{slug}/records", s.auth(s.createRecord))
	s.Mux.HandleFunc("GET /api/apps/{id}/collections/{slug}/records/{recordId}", s.auth(s.getRecord))
	s.Mux.HandleFunc("PATCH /api/apps/{id}/collections/{slug}/records/{recordId}", s.auth(s.updateRecord))
	s.Mux.HandleFunc("DELETE /api/apps/{id}/collections/{slug}/records/{recordId}", s.auth(s.deleteRecord))
	s.Mux.HandleFunc("GET /api/apps/{id}/collections/{slug}/records/{recordId}/files/{fieldName}", s.auth(s.recordFile))
	s.Mux.HandleFunc("GET /api/apps/{id}/access", s.auth(s.getAppAccess))
	s.Mux.HandleFunc("GET /api/apps/{id}/members", s.auth(s.listAppMemberChoices))
	s.Mux.HandleFunc("PUT /api/apps/{id}/access", s.auth(s.updateAppAccess))
	s.Mux.HandleFunc("POST /api/apps/{id}/visit", s.auth(s.visitApp))
}

func (s *Server) routesCatalog() {
	s.Mux.HandleFunc("GET /api/apps/{id}/backend/catalog", s.auth(s.getBackendCatalog))
	s.Mux.HandleFunc("GET /api/apps/{id}/backend/spec", s.auth(s.getBackendSpec))
}

func (s *Server) routesBackendPlans() {
	s.Mux.HandleFunc("GET /api/apps/{id}/backend/candidates", s.auth(s.listBackendPlanCandidates))
	s.Mux.HandleFunc("POST /api/apps/{id}/backend/plans", s.auth(s.createBackendPlan))
	s.Mux.HandleFunc("GET /api/apps/{id}/backend/plans/{planId}", s.auth(s.getBackendPlan))
	s.Mux.HandleFunc("POST /api/apps/{id}/backend/plans/{planId}/apply", s.auth(s.applyBackendPlan))
}

func (s *Server) routesVersions() {
	s.Mux.HandleFunc("GET /api/apps/{id}/runtime", s.auth(s.publishedRuntime))
	s.Mux.HandleFunc("GET /api/apps/{id}/runtime/stream", s.auth(s.streamPublishedRuntime))
	s.Mux.HandleFunc("GET /api/apps/{id}/versions", s.auth(s.listVersions))
	s.Mux.HandleFunc("GET /api/apps/{id}/versions/{versionId}", s.auth(s.getVersion))
	s.Mux.HandleFunc("POST /api/apps/{id}/versions/{versionId}/activate", s.auth(s.activateVersion))
	s.Mux.HandleFunc("GET /api/apps/{id}/versions/{versionId}/preview", s.auth(s.previewVersion))
	s.Mux.HandleFunc("GET /api/apps/{id}/versions/{versionId}/validation", s.auth(s.validateVersion))
	s.Mux.HandleFunc("GET /api/apps/{id}/versions/{versionId}/diff", s.auth(s.diffVersion))
	s.Mux.HandleFunc("POST /api/apps/{id}/runtime/actions/{actionId}", s.auth(s.runRuntimeAction))
	s.Mux.HandleFunc("POST /api/apps/{id}/versions/preview", s.auth(s.previewUIDefinition))
}

func (s *Server) routesFiles() {
	s.Mux.HandleFunc("GET /api/apps/{id}/files", s.auth(s.listAppFiles))
	s.Mux.HandleFunc("POST /api/apps/{id}/files", s.auth(s.createAppFile))
	s.Mux.HandleFunc("GET /api/apps/{id}/files/{fileId}/content", s.auth(s.appFileContent))
	s.Mux.HandleFunc("POST /api/apps/{id}/files/{fileId}/attach", s.auth(s.attachAppFile))
	s.Mux.HandleFunc("GET /api/apps/{id}/files/{fileId}/download", s.auth(s.downloadAppFile))
}

func (s *Server) routesOperations() {
	s.Mux.HandleFunc("POST /api/apps/{id}/query", s.auth(s.queryRecords))
	s.Mux.HandleFunc("POST /api/apps/{id}/batch-plans", s.auth(s.createBatchPlan))
	s.Mux.HandleFunc("GET /api/apps/{id}/batch-plans/{jobId}", s.auth(s.getBatchPlan))
	s.Mux.HandleFunc("POST /api/apps/{id}/batch-plans/{jobId}/commit", s.auth(s.commitBatchPlan))
	s.Mux.HandleFunc("POST /api/apps/{id}/import-plans", s.auth(s.createImportPlan))
	s.Mux.HandleFunc("GET /api/apps/{id}/import-plans/{planId}", s.auth(s.getImportPlan))
	s.Mux.HandleFunc("POST /api/apps/{id}/import-plans/{planId}/commit", s.auth(s.commitImportPlan))
}
