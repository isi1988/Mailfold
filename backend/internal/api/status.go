package api

import "net/http"

// registerStatusRoutes wires the read-only HTTP routes that report the health
// and status of the mailcow deployment. It is defined as its own function so
// the status feature owns its route wiring and registers alongside the other
// feature groups.
//
// Three status endpoints are exposed, each protected by requireAuth:
//   - "GET /api/status/containers" reports the state of the mailcow Docker
//     containers.
//   - "GET /api/status/version" reports the running mailcow version.
//   - "GET /api/status/vmail" reports mail storage (vmail volume) usage.
//
// These handlers deliberately bypass the generic serveRaw helper and use
// writeUpstream directly so the exact upstream mailcow JSON is proxied through
// unchanged.
func (s *Server) registerStatusRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/status/containers", s.requireAuth(s.handleContainers))
	mux.HandleFunc("GET /api/status/version", s.requireAuth(s.handleVersion))
	mux.HandleFunc("GET /api/status/vmail", s.requireAuth(s.handleVmail))
	mux.HandleFunc("GET /api/status/server", s.requireAuth(s.handleServer))
}

// handleServer reports the configured public mail-server hostname (from
// MAILFOLD_SERVER_NAME) so the UI can label its status indicator instead of a
// placeholder. Unlike the other status handlers it reads local configuration
// rather than proxying mailcow, and returns an empty name when unconfigured.
func (s *Server) handleServer(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"name": s.cfg.ServerName})
}

// handleContainers serves the current state of the mailcow Docker containers.
// It exists to proxy mailcow's container status report to authenticated callers
// so operators can see which services are running. On an upstream failure it
// responds with 502 Bad Gateway (the error originates from the downstream
// mailcow service, not from a bad client request); on success it forwards the
// raw upstream JSON unchanged.
func (s *Server) handleContainers(w http.ResponseWriter, r *http.Request) {
	raw, err := s.mc.Containers(r.Context())
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeUpstream(w, raw)
}

// handleVersion serves the running mailcow version. It exists so clients can
// display or verify which mailcow release is deployed. On an upstream failure it
// responds with 502 Bad Gateway; on success it forwards the raw upstream JSON
// unchanged.
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	raw, err := s.mc.Version(r.Context())
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeUpstream(w, raw)
}

// handleVmail serves mailcow's vmail storage usage report. It exists so
// operators can monitor how much of the mail storage volume is consumed. On an
// upstream failure it responds with 502 Bad Gateway; on success it forwards the
// raw upstream JSON unchanged.
func (s *Server) handleVmail(w http.ResponseWriter, r *http.Request) {
	raw, err := s.mc.Vmail(r.Context())
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeUpstream(w, raw)
}
