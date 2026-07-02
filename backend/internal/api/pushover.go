package api

import "net/http"

// registerPushoverRoutes wires the HTTP route for mailcow's per-mailbox Pushover
// notification settings. mailcow exposes no read endpoint for Pushover, so only
// the collection-level PUT (edit) is registered; registerCRUD wires it to
// EditPushover under requireAuth.
func (s *Server) registerPushoverRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/pushover", crud{edit: s.mc.EditPushover})
}
