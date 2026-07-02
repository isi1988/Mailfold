package api

import "net/http"

// registerRateLimitDomainRoutes attaches the "/api/ratelimits/domain" endpoints
// to mux.
//
// A domain rate limit caps how much outbound mail a whole domain may send in a
// given window. mailcow only supports reading and editing these limits — there
// is no add or delete verb, since the limit exists implicitly for every domain
// — so only GET and PUT are wired. The raw field streams s.mc.RateLimitDomains'
// pre-encoded JSON straight through, and the edit field binds PUT to
// EditRateLimitDomain. All routes are protected by requireAuth (applied inside
// registerCRUD).
func (s *Server) registerRateLimitDomainRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/ratelimits/domain", crud{
		raw:  s.mc.RateLimitDomains,    // GET lists per-domain rate limits; already JSON, streamed verbatim.
		edit: s.mc.EditRateLimitDomain, // PUT updates a domain's rate limit.
	})
}
