package api

import "net/http"

// registerQuarantineRoutes wires the HTTP routes for mailcow's spam quarantine.
// It is defined as its own function so the quarantine feature owns its route
// wiring and can be registered alongside the other feature groups.
//
// Two operations are exposed through registerCRUD:
//   - A read-only GET on "/api/quarantine" that returns the raw list of
//     quarantined messages from mailcow.
//   - A DELETE on "/api/quarantine" that removes the quarantined items named in
//     the request body via DeleteQuarantine.
//
// No create or edit verbs are wired because quarantined items are produced by
// mailcow's spam filtering, not by this API; callers may only inspect or delete
// them. Both routes are protected by requireAuth (applied inside registerCRUD).
func (s *Server) registerQuarantineRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/quarantine", crud{raw: s.mc.Quarantine, del: s.mc.DeleteQuarantine})
}
