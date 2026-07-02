package api

import "net/http"

// registerFilterRoutes wires the Sieve filter CRUD endpoints under /api/filters.
func (s *Server) registerFilterRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/filters", crud{
		raw:    s.mc.Filters,
		create: s.mc.AddFilter,
		edit:   s.mc.EditFilter,
		del:    s.mc.DeleteFilter,
	})
}
