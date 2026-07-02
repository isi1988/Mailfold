package api

import "net/http"

// registerBCCRoutes attaches the "/api/bcc" CRUD endpoints to mux.
//
// A BCC map is a mailcow-managed rule that silently sends a copy of a mailbox or
// domain's mail to another address, which lets Mailfold configure archiving or
// compliance recipients. The listing uses the raw field because s.mc.BCCMaps
// already returns pre-encoded JSON, so it is streamed straight through instead
// of being re-marshalled. BCC maps have no edit verb in mailcow, so only create
// and del are bound.
func (s *Server) registerBCCRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/bcc", crud{
		raw:    s.mc.BCCMaps,   // GET lists BCC maps; already JSON, streamed verbatim.
		create: s.mc.AddBCC,    // POST creates a new BCC map.
		del:    s.mc.DeleteBCC, // DELETE removes a BCC map.
	})
}
