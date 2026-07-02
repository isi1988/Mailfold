// Package api exposes Mailfold's HTTP surface on top of the mailcow client.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
)

// Server wires HTTP routes to the mailcow API and the authenticator. It holds
// its collaborators through interfaces/values so the transport layer stays
// decoupled from concrete implementations.
type Server struct {
	cfg          *config.Config
	mc           Mailcow
	auth         *auth.Authenticator
	loginLimiter *ratelimit.Limiter
	logger       *slog.Logger
}

// NewServer constructs a Server from its collaborators. mc is any type
// satisfying the Mailcow interface (in production, *mailcow.Client). limiter
// throttles login attempts per client IP.
func NewServer(cfg *config.Config, mc Mailcow, authn *auth.Authenticator, limiter *ratelimit.Limiter, logger *slog.Logger) *Server {
	return &Server{cfg: cfg, mc: mc, auth: authn, loginLimiter: limiter, logger: logger}
}

// Handler builds the HTTP handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/health/ready", s.handleReady)

	// API documentation (public).
	s.registerDocsRoutes(mux)

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
	s.registerDomainAdminRoutes(mux)
	s.registerResourceRoutes(mux)
	s.registerAppPasswordRoutes(mux)
	s.registerOAuth2Routes(mux)
	s.registerForwardingHostRoutes(mux)
	s.registerTransportRoutes(mux)
	s.registerRelayhostRoutes(mux)
	s.registerTLSPolicyRoutes(mux)
	s.registerBCCRoutes(mux)
	s.registerRecipientMapRoutes(mux)

	// Static SPA (served only when a build directory is present).
	s.registerFrontend(mux)

	return s.withCommon(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "mailfold"})
}

// handleReady is a readiness probe: it returns 200 only when the upstream
// mailcow API is reachable, so an orchestrator can withhold traffic until the
// backend can actually serve requests. A short timeout keeps a slow or dead
// mailcow from hanging the probe.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if _, err := s.mc.Version(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
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
