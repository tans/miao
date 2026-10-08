package httpapi

// registerRoutes is the only top-level route composer. Each group keeps its
// endpoint registration close to the domain handlers it owns.
func (s *Server) registerRoutes() {
	s.registerCoreRoutes()
	s.registerIdentityRoutes()
	s.registerApplicationRoutes()
	s.registerExecutionRoutes()
	s.registerAutomationRoutes()
	s.registerPublicRoutes()
	s.routesAdmin()
	s.registerFallbackRoutes()
}
