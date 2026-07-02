// Package api exposes Mailfold's HTTP surface on top of the mailcow client.
package api

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
)

// Server wires HTTP routes to the mailcow client.
type Server struct {
	cfg    *config.Config
	mc     *mailcow.Client
	logger *slog.Logger
}

// NewServer constructs a Server.
func NewServer(cfg *config.Config, mc *mailcow.Client, logger *slog.Logger) *Server {
	return &Server{cfg: cfg, mc: mc, logger: logger}
}

// Handler builds the HTTP handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/domains", s.handleDomains)
	mux.HandleFunc("GET /api/mailboxes", s.handleMailboxes)

	// Serve the built frontend (SPA) if a build directory is present.
	s.registerFrontend(mux)

	return s.withLogging(mux)
}

// withLogging logs one line per request.
func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// registerFrontend serves static SPA assets with an index.html fallback for
// client-side routes. The UI comes from a separate design project, so this
// only activates when a build directory exists.
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
