package httpapi

func (s *Server) registerIdentityRoutes() {
	s.registerAuthRoutes()
}

func (s *Server) registerApplicationRoutes() {
	s.routesApps()
	s.routesCatalog()
	s.routesBackendPlans()
	s.routesVersions()
	s.routesFiles()
	s.routesOperations()
}

func (s *Server) registerExecutionRoutes() {
	s.routesAgent()
}

func (s *Server) registerAutomationRoutes() {
	s.routesAutomations()
	s.routesTasks()
	s.routesBusinessActions()
	s.routesWorkflows()
	s.routesConnectors()
	s.routesCollectionScripts()
}

func (s *Server) registerPublicRoutes() {
	s.routesPublications()
}
