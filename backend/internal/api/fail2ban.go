package api

import "net/http"

// registerFail2BanRoutes wires the HTTP routes for managing mailcow's fail2ban
// configuration. It lives in its own function so the fail2ban feature owns its
// route wiring and registers cleanly alongside the other feature groups.
//
// The routes provide:
//   - A read-only GET on "/api/fail2ban" that returns the current fail2ban
//     configuration (ban time, retry limits, allow/deny lists, etc.) verbatim
//     from mailcow.
//   - A "PUT /api/fail2ban" endpoint that updates that configuration.
//
// Both routes require authentication so only authorized operators can view or
// change the intrusion-prevention settings.
func (s *Server) registerFail2BanRoutes(mux *http.ServeMux) {
	// Expose the current fail2ban settings as a raw JSON passthrough. Only the
	// raw GET is registered here; the update verb is wired separately below
	// because fail2ban is edited (not created) through a single config object.
	s.registerCRUD(mux, "/api/fail2ban", crud{raw: s.mc.Fail2Ban})
	// EditFail2Ban has the same (ctx, attr) shape as a create operation, so the
	// PUT handler reuses serveCreate to decode the request body and forward the
	// attributes to mailcow. It is registered explicitly (rather than via
	// crud.edit) so the update maps to PUT while sharing the create plumbing.
	mux.HandleFunc("PUT /api/fail2ban", s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		s.serveCreate(w, r, s.mc.EditFail2Ban)
	}))
}
