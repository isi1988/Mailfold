package api

import "net/http"

// registerRateLimitMailboxRoutes attaches the "/api/ratelimits/mailbox" routes
// to mux.
//
// A mailbox rate limit caps how many messages a single mailbox may send within
// a rolling window, letting Mailfold contain compromised or misbehaving
// accounts. The listing uses the raw field because s.mc.RateLimitMailboxes
// already returns pre-encoded JSON, so it is streamed straight through instead
// of being re-marshalled. mailcow exposes no add or delete verb for these
// entries (they are edited in place), so only GET and PUT are wired via the raw
// and edit fields. All routes are protected by requireAuth (applied inside
// registerCRUD).
func (s *Server) registerRateLimitMailboxRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/ratelimits/mailbox", crud{
		raw:  s.mc.RateLimitMailboxes,   // GET lists mailbox rate limits; already JSON, streamed verbatim.
		edit: s.mc.EditRateLimitMailbox, // PUT updates a mailbox rate limit.
	})
}
