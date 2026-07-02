package api

import "net/http"

// registerDomainAdminRoutes attaches the "/api/domain-admins" CRUD endpoints to mux.
//
// A domain administrator is a scoped mailcow account permitted to manage a subset
// of domains without full super-admin rights, so Mailfold exposes it as its own
// route group registered at server startup. Its listing uses the raw field rather
// than list: s.mc.DomainAdmins already returns pre-encoded JSON, so it is handed to
// the helper as-is and streamed straight through instead of being re-marshalled.
// The create/edit/del fields bind the remaining verbs to the matching mailcow calls.
func (s *Server) registerDomainAdminRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/domain-admins", crud{
		raw:    s.mc.DomainAdmins,      // GET lists domain admins; already JSON, streamed verbatim.
		create: s.mc.AddDomainAdmin,    // POST creates a new domain administrator.
		edit:   s.mc.EditDomainAdmin,   // PUT/PATCH updates an existing domain admin's settings.
		del:    s.mc.DeleteDomainAdmin, // DELETE removes a domain administrator.
	})
}
