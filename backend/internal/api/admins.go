package api

import "net/http"

// registerAdminRoutes attaches the "/api/admins" CRUD endpoints to mux.
//
// An admin is a mailcow super-administrator with full control over every domain,
// so Mailfold exposes it as its own route group registered at server startup.
// Its listing uses the raw field rather than list: s.mc.Admins already returns
// pre-encoded JSON, so it is handed to the helper as-is and streamed straight
// through instead of being re-marshalled. The create/edit/del fields bind the
// remaining verbs to the matching mailcow calls.
func (s *Server) registerAdminRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/admins", crud{
		raw:    s.mc.Admins,      // GET lists admins; already JSON, streamed verbatim.
		create: s.mc.AddAdmin,    // POST creates a new super-administrator.
		edit:   s.mc.EditAdmin,   // PUT/PATCH updates an existing admin's settings.
		del:    s.mc.DeleteAdmin, // DELETE removes a super-administrator.
	})
}
