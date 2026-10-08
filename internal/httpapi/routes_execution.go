package httpapi

func (s *Server) routesAgent() {
	s.Mux.HandleFunc("GET /api/agent/threads", s.auth(s.listThreads))
	s.Mux.HandleFunc("POST /api/agent/threads", s.auth(s.createThread))
	s.Mux.HandleFunc("GET /api/agent/threads/{threadId}/messages", s.auth(s.listThreadMessages))
	s.Mux.HandleFunc("POST /api/agent/threads/{threadId}/messages", s.auth(s.createThreadMessage))
	s.Mux.HandleFunc("GET /api/agent/conversation", s.auth(s.getConversation))
	s.Mux.HandleFunc("PUT /api/agent/conversation", s.auth(s.saveConversation))
	s.Mux.HandleFunc("DELETE /api/agent/conversation", s.auth(s.clearConversation))
	s.Mux.HandleFunc("GET /api/apps/{id}/context", s.auth(s.getAppContext))
	s.Mux.HandleFunc("PUT /api/apps/{id}/context", s.auth(s.saveAppContext))
	s.Mux.HandleFunc("GET /api/apps/{id}/record-changes", s.auth(s.recordChanges))
	s.Mux.HandleFunc("POST /api/apps/{id}/record-changes/{changeId}/restore", s.auth(s.restoreRecordChange))
	s.routesHarness()
}

func (s *Server) routesHarness() {
	s.Mux.HandleFunc("POST /api/agent/runs", s.auth(s.submitHarnessRun))
	s.Mux.HandleFunc("GET /api/agent/runs/{runId}", s.auth(s.getHarnessRun))
	s.Mux.HandleFunc("GET /api/agent/runs/{runId}/events", s.auth(s.getHarnessEvents))
	s.Mux.HandleFunc("GET /api/agent/runs/{runId}/stream", s.auth(s.streamHarnessRun))
	s.Mux.HandleFunc("POST /api/agent/runs/{runId}/confirm", s.auth(s.confirmHarnessRun))
	s.Mux.HandleFunc("POST /api/agent/runs/{runId}/cancel", s.auth(s.cancelHarnessRun))
	s.Mux.HandleFunc("POST /api/agent/runs/{runId}/continue", s.auth(s.continueHarnessRun))
	s.Mux.HandleFunc("POST /api/agent/runs/{runId}/resume", s.auth(s.resumeHarnessRun))
}
