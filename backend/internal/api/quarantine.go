package api

import "net/http"

// registerQuarantineRoutes wires the HTTP routes for mailcow's spam quarantine.
// It is defined as its own function so the quarantine feature owns its route
// wiring and can be registered alongside the other feature groups.
//
// Three operations are exposed through registerCRUD:
//   - A read-only GET on "/api/quarantine" that returns the raw list of
//     quarantined messages from mailcow.
//   - A PUT on "/api/quarantine" that applies an action (for example
//     {"action":"release"} to deliver a held message) to the items in the
//     request body via EditQuarantine.
//   - A DELETE on "/api/quarantine" that removes the quarantined items named in
//     the request body via DeleteQuarantine.
//
// No create verb is wired because quarantined items are produced by mailcow's
// spam filtering, not by this API. All routes are protected by requireAuth
// (applied inside registerCRUD).
func (s *Server) registerQuarantineRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/quarantine", crud{raw: s.mc.Quarantine, edit: s.mc.EditQuarantine, del: s.mc.DeleteQuarantine})
}
