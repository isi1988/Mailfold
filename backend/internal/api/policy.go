package api

import (
	"context"
	"encoding/json"
	"net/http"
)

// registerPolicyRoutes wires the HTTP routes for mailcow's allow/deny policy
// lists (the sender allowlist and blocklist). It is kept in its own function so
// the policy feature owns its route wiring and registers alongside the other
// feature groups.
//
// The routes provide:
//   - "POST /api/policy" to add a policy entry and "DELETE /api/policy" to
//     remove entries, both wired through registerCRUD. There is intentionally no
//     GET on the base path because policy entries are always read scoped to a
//     specific domain via the two routes below.
//   - "GET /api/policy/allow/{domain}" to list the allowlist entries for a
//     given domain.
//   - "GET /api/policy/deny/{domain}" to list the blocklist entries for a given
//     domain.
//
// All routes require authentication so only authorized operators can view or
// change sender policy.
func (s *Server) registerPolicyRoutes(mux *http.ServeMux) {
	// Wire creation and deletion of policy entries. Only create and del are
	// provided; the per-domain reads are handled by the dedicated routes below.
	s.registerCRUD(mux, "/api/policy", crud{create: s.mc.AddPolicy, del: s.mc.DeletePolicy})
	// Return the allowlist (whitelisted senders) for the domain named in the
	// {domain} path segment, streamed straight from mailcow.
	mux.HandleFunc("GET /api/policy/allow/{domain}", s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		s.serveRaw(w, r, func(ctx context.Context) (json.RawMessage, error) {
			return s.mc.PolicyAllow(ctx, r.PathValue("domain"))
		})
	}))
	// Return the blocklist (blacklisted senders) for the domain named in the
	// {domain} path segment, streamed straight from mailcow.
	mux.HandleFunc("GET /api/policy/deny/{domain}", s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		s.serveRaw(w, r, func(ctx context.Context) (json.RawMessage, error) { return s.mc.PolicyDeny(ctx, r.PathValue("domain")) })
	}))
}
