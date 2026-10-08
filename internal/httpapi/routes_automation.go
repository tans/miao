package httpapi

func (s *Server) routesAutomations() {
	s.Mux.HandleFunc("GET /api/apps/{id}/automations", s.auth(s.listAutomations))
	s.Mux.HandleFunc("POST /api/apps/{id}/automations", s.auth(s.createAutomation))
	s.Mux.HandleFunc("POST /api/apps/{id}/automations/{ruleId}/enable", s.auth(s.enableAutomation))
	s.Mux.HandleFunc("GET /api/notifications", s.auth(s.listNotifications))
	s.Mux.HandleFunc("POST /api/notifications/{notificationId}/read", s.auth(s.readNotification))
}

func (s *Server) routesTasks() {
	s.Mux.HandleFunc("GET /api/apps/{id}/tasks", s.auth(s.listTasks))
	s.Mux.HandleFunc("POST /api/apps/{id}/tasks", s.auth(s.createTask))
	s.Mux.HandleFunc("POST /api/apps/{id}/tasks/from-agent", s.auth(s.createTaskFromAgent))
	s.Mux.HandleFunc("PATCH /api/apps/{id}/tasks/{taskId}", s.auth(s.updateTask))
	for _, action := range []string{"enable", "pause", "preview", "archive", "restore", "transfer", "events", "run"} {
		s.Mux.HandleFunc("POST /api/apps/{id}/tasks/{taskId}/"+action, s.auth(s.taskAction))
	}
	s.Mux.HandleFunc("GET /api/apps/{id}/runs", s.auth(s.listRuns))
	s.Mux.HandleFunc("GET /api/apps/{id}/runs/{runId}", s.auth(s.getRun))
	for _, action := range []string{"cancel", "retry", "resolve"} {
		s.Mux.HandleFunc("POST /api/apps/{id}/runs/{runId}/"+action, s.auth(s.runAction))
	}
}

func (s *Server) routesBusinessActions() {
	s.Mux.HandleFunc("GET /api/apps/{id}/actions", s.auth(s.listBusinessActions))
	s.Mux.HandleFunc("POST /api/apps/{id}/actions", s.auth(s.createBusinessAction))
	s.Mux.HandleFunc("PATCH /api/apps/{id}/actions/{actionId}", s.auth(s.updateBusinessAction))
	s.Mux.HandleFunc("POST /api/apps/{id}/actions/{actionId}/enable", s.auth(s.enableBusinessAction))
	s.Mux.HandleFunc("POST /api/apps/{id}/actions/{actionId}/execute", s.auth(s.executeBusinessAction))
}

func (s *Server) routesWorkflows() {
	s.Mux.HandleFunc("GET /api/apps/{id}/workflows", s.auth(s.listWorkflows))
	s.Mux.HandleFunc("POST /api/apps/{id}/workflows", s.auth(s.createWorkflow))
	s.Mux.HandleFunc("PATCH /api/apps/{id}/workflows/{workflowId}", s.auth(s.updateWorkflow))
	s.Mux.HandleFunc("POST /api/apps/{id}/workflows/{workflowId}/enable", s.auth(s.enableWorkflow))
	s.Mux.HandleFunc("POST /api/apps/{id}/workflows/{workflowId}/transition", s.auth(s.transitionWorkflow))
}

func (s *Server) routesConnectors() {
	s.Mux.HandleFunc("GET /api/apps/{id}/connectors", s.auth(s.listConnectors))
	s.Mux.HandleFunc("POST /api/apps/{id}/connectors", s.auth(s.createConnector))
	s.Mux.HandleFunc("PATCH /api/apps/{id}/connectors/{connectorId}", s.auth(s.updateConnector))
	s.Mux.HandleFunc("POST /api/apps/{id}/connectors/{connectorId}/enable", s.auth(s.enableConnector))
	s.Mux.HandleFunc("POST /api/apps/{id}/connectors/{connectorId}/fetch", s.auth(s.fetchConnector))
}

func (s *Server) routesCollectionScripts() {
	s.Mux.HandleFunc("GET /api/apps/{id}/collection-scripts", s.auth(s.listCollectionScripts))
	s.Mux.HandleFunc("POST /api/apps/{id}/collection-scripts", s.auth(s.createCollectionScript))
	s.Mux.HandleFunc("PATCH /api/apps/{id}/collection-scripts/{scriptId}", s.auth(s.updateCollectionScript))
	for _, action := range []string{"enable", "pause", "preview", "run"} {
		s.Mux.HandleFunc("POST /api/apps/{id}/collection-scripts/{scriptId}/"+action, s.auth(s.collectionScriptAction))
	}
	s.Mux.HandleFunc("GET /api/apps/{id}/collection-scripts/{scriptId}/runs", s.auth(s.listCollectionScriptRuns))
	s.Mux.HandleFunc("GET /api/apps/{id}/collection-scripts/{scriptId}/runs/{runId}", s.auth(s.getCollectionScriptRun))
	s.Mux.HandleFunc("POST /api/apps/{id}/collection-scripts/{scriptId}/runs/{runId}/retry-notifications", s.auth(s.retryCollectionRunNotifications))
}
