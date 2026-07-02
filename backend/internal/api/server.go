// Package api exposes Mailfold's HTTP surface on top of the mailcow client.
package api

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
)

// Server wires HTTP routes to the mailcow API and the authenticator. It holds
// its collaborators through interfaces/values so the transport layer stays
// decoupled from concrete implementations.
type Server struct {
	cfg    *config.Config
	mc     Mailcow
	auth   *auth.Authenticator
	logger *slog.Logger
}

// NewServer constructs a Server from its collaborators. mc is any type
// satisfying the Mailcow interface (in production, *mailcow.Client).
func NewServer(cfg *config.Config, mc Mailcow, authn *auth.Authenticator, logger *slog.Logger) *Server {
	return &Server{cfg: cfg, mc: mc, auth: authn, logger: logger}
}

// Handler builds the HTTP handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)

	// Authentication.
	s.registerAuthRoutes(mux)

	// Management resources (all require authentication).
	s.registerStatusRoutes(mux)
	s.registerDomainRoutes(mux)
	s.registerMailboxRoutes(mux)
	s.registerAliasRoutes(mux)
	s.registerDKIMRoutes(mux)
	s.registerSyncJobRoutes(mux)
	s.registerQueueRoutes(mux)
	s.registerLogRoutes(mux)
	s.registerFail2BanRoutes(mux)
	s.registerQuarantineRoutes(mux)
	s.registerPolicyRoutes(mux)

	// Static SPA (served only when a build directory is present).
	s.registerFrontend(mux)

	return s.withCommon(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "mailfold"})
}

// registerFrontend serves static SPA assets with an index.html fallback for
// client-side routes. The UI comes from a separate design project, so this only
// activates when a build directory exists.
func (s *Server) registerFrontend(mux *http.ServeMux) {
	dir := s.cfg.FrontendDir
	if dir == "" {
		return
	}
	if _, err := os.Stat(dir); err != nil {
		s.logger.Info("frontend build not found; serving API only", "dir", dir)
		return
	}

	fileServer := http.FileServer(http.Dir(dir))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean(r.URL.Path)
		if _, err := os.Stat(filepath.Join(dir, clean)); os.IsNotExist(err) && !strings.HasPrefix(r.URL.Path, "/api/") {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
