package api

import "net/http"

// registerTransportRoutes attaches the "/api/transports" CRUD endpoints to mux.
//
// A transport map is a mailcow-managed Postfix routing rule that controls how
// mail for a particular destination is relayed (for example via a smarthost with
// its own credentials). Its listing uses the raw field rather than list because
// s.mc.Transports already returns pre-encoded JSON, so it is handed to the helper
// as-is and streamed straight through instead of being re-marshalled. The
// create/edit/del fields bind the remaining verbs to the matching mailcow calls.
func (s *Server) registerTransportRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/transports", crud{
		raw:    s.mc.Transports,      // GET lists transport maps; already JSON, streamed verbatim.
		create: s.mc.AddTransport,    // POST creates a new transport map.
		edit:   s.mc.EditTransport,   // PUT/PATCH updates an existing transport map.
		del:    s.mc.DeleteTransport, // DELETE removes a transport map.
	})
}
