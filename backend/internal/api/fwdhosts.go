package api

import "net/http"

// registerForwardingHostRoutes attaches the "/api/forwarding-hosts" CRUD
// endpoints to mux.
//
// A forwarding host is a trusted upstream relay whose inbound mail mailcow
// accepts without applying spam or greylist filtering, which lets Mailfold
// whitelist known senders such as external gateways. The listing uses the raw
// field because s.mc.ForwardingHosts already returns pre-encoded JSON, so it is
// streamed straight through instead of being re-marshalled. Forwarding hosts
// have no edit verb in mailcow, so only create and del are bound.
func (s *Server) registerForwardingHostRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/forwarding-hosts", crud{
		raw:    s.mc.ForwardingHosts,      // GET lists forwarding hosts; already JSON, streamed verbatim.
		create: s.mc.AddForwardingHost,    // POST registers a new forwarding host.
		del:    s.mc.DeleteForwardingHost, // DELETE removes a forwarding host.
	})
}
