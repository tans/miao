package httpapi

func (s *Server) routesPublications() {
	s.Mux.HandleFunc("GET /api/apps/{id}/publication", s.auth(s.getPublication))
	s.Mux.HandleFunc("GET /api/public/{slug}/runtime", s.publicRuntime)
	s.Mux.HandleFunc("POST /api/public/{slug}/visit", s.publicVisit)
	s.Mux.HandleFunc("GET /api/public/{slug}/records", s.publicRecords)
	s.Mux.HandleFunc("GET /api/public/{slug}/records/{itemSlug}", s.publicRecordDetail)
	s.Mux.HandleFunc("GET /api/public/{slug}/images/{pageId}/{source}/{table}/{recordId}/{field}", s.publicImage)
}
