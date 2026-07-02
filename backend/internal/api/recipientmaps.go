package api

import "net/http"

// registerRecipientMapRoutes attaches the "/api/recipient-maps" CRUD endpoints
// to mux.
//
// A recipient map is a mailcow-managed rewrite rule that redirects mail addressed
// to one recipient towards a different address before delivery. The listing uses
// the raw field because s.mc.RecipientMaps already returns pre-encoded JSON, so it
// is streamed straight through instead of being re-marshalled. Recipient maps have
// no edit verb in mailcow, so only create and del are bound.
func (s *Server) registerRecipientMapRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/recipient-maps", crud{
		raw:    s.mc.RecipientMaps,      // GET lists recipient maps; already JSON, streamed verbatim.
		create: s.mc.AddRecipientMap,    // POST creates a new recipient map.
		del:    s.mc.DeleteRecipientMap, // DELETE removes a recipient map.
	})
}
