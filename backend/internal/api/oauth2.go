package api

import "net/http"

// registerOAuth2Routes wires the HTTP routes for mailcow's OAuth2 clients.
// It is defined as its own function so the OAuth2 feature owns its route wiring
// and can be registered alongside the other feature groups at server startup.
//
// Three operations are exposed through registerCRUD on "/api/oauth2-clients":
//   - A read-only GET that returns the raw list of OAuth2 clients from mailcow
//     via OAuth2Clients (streamed verbatim through the raw field).
//   - A POST that registers a new client via AddOAuth2Client.
//   - A DELETE that removes the clients named in the request body via
//     DeleteOAuth2Client.
//
// No edit verb is wired because mailcow does not expose an OAuth2 client edit
// endpoint; callers recreate a client to change it. All routes are protected by
// requireAuth (applied inside registerCRUD).
func (s *Server) registerOAuth2Routes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/oauth2-clients", crud{
		raw:    s.mc.OAuth2Clients,
		create: s.mc.AddOAuth2Client,
		del:    s.mc.DeleteOAuth2Client,
	})
}
