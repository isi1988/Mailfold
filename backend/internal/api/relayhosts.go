package api

import "net/http"

// registerRelayhostRoutes attaches the "/api/relayhosts" CRUD endpoints to mux.
//
// A relayhost is an upstream smarthost that mailcow routes outbound mail
// through, which lets Mailfold send via an external provider instead of
// delivering directly. The listing uses the raw field because s.mc.Relayhosts
// already returns pre-encoded JSON, so it is handed to the helper as-is and
// streamed straight through instead of being re-marshalled. The create/edit/del
// fields bind the remaining verbs to the matching mailcow calls.
func (s *Server) registerRelayhostRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/relayhosts", crud{
		raw:    s.mc.Relayhosts,      // GET lists relayhosts; already JSON, streamed verbatim.
		create: s.mc.AddRelayhost,    // POST creates a new relayhost.
		edit:   s.mc.EditRelayhost,   // PUT/PATCH updates an existing relayhost's settings.
		del:    s.mc.DeleteRelayhost, // DELETE removes a relayhost.
	})
}
