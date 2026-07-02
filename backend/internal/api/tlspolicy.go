package api

import "net/http"

// registerTLSPolicyRoutes wires the HTTP routes for mailcow's TLS policy maps,
// which pin the outbound SMTP encryption behaviour for individual destinations.
// It is defined as its own function so the TLS policy feature owns its route
// wiring and can be registered alongside the other feature groups.
//
// Three operations are exposed through registerCRUD on "/api/tls-policies":
//   - A read-only GET that returns the raw list of TLS policy map entries from
//     mailcow.
//   - A POST that adds a policy entry via AddTLSPolicy.
//   - A DELETE that removes the entries named in the request body via
//     DeleteTLSPolicy.
//
// No edit verb is wired because mailcow has no edit endpoint for TLS policy
// maps; entries are replaced by deleting and re-adding them. All routes are
// protected by requireAuth (applied inside registerCRUD).
func (s *Server) registerTLSPolicyRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/tls-policies", crud{raw: s.mc.TLSPolicies, create: s.mc.AddTLSPolicy, del: s.mc.DeleteTLSPolicy})
}
