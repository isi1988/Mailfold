package api

import "net/http"

// registerResourceRoutes attaches the "/api/resources" CRUD endpoints to mux.
//
// A resource is a mailcow-managed shared object — such as a meeting room or a piece
// of bookable equipment — represented as a special mailbox that calendar clients can
// reserve. The listing uses the raw field because s.mc.Resources already returns
// pre-encoded JSON, so it is streamed straight through instead of being re-marshalled.
// The create/edit/del fields bind the remaining verbs to the matching mailcow calls.
func (s *Server) registerResourceRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/resources", crud{
		raw:    s.mc.Resources,      // GET lists resources; already JSON, streamed verbatim.
		create: s.mc.AddResource,    // POST creates a new resource.
		edit:   s.mc.EditResource,   // PUT/PATCH updates an existing resource's settings.
		del:    s.mc.DeleteResource, // DELETE removes a resource.
	})
}
