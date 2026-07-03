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

	"github.com/isi1988/Mailfold/backend/internal/apikey"
	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/dav"
	"github.com/isi1988/Mailfold/backend/internal/metrics"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
	"github.com/isi1988/Mailfold/backend/internal/webmail"
)

// Server wires HTTP routes to the mailcow API and the authenticator. It holds
// its collaborators through interfaces/values so the transport layer stays
// decoupled from concrete implementations.
type Server struct {
	cfg             *config.Config
	mc              Mailcow
	auth            *auth.Authenticator
	loginLimiter    *ratelimit.Limiter
	metrics         *metrics.Metrics
	webmail         *webmail.Client
	webmailSessions *webmail.Sessions
	davStore        *dav.Store
	davAuth         *davVerifier
	apikeyStore     *apikey.Store
	apikeyCipher    *apikey.Cipher
	apikeyKeyLimit  *ratelimit.Limiter // per-key request budget
	apikeyIPLimit   *ratelimit.Limiter // pre-auth per-IP guard against token guessing
	logger          *slog.Logger
}

// NewServer constructs a Server from its collaborators. mc is any type
// satisfying the Mailcow interface (in production, *mailcow.Client). limiter
// throttles login attempts per client IP. The webmail client and session store
// are built from cfg (webmail is disabled when no IMAP address is configured).
func NewServer(cfg *config.Config, mc Mailcow, authn *auth.Authenticator, limiter *ratelimit.Limiter, logger *slog.Logger) *Server {
	wm := webmail.NewClient(cfg.IMAPAddr, cfg.SMTPAddr, cfg.MailUseTLS, cfg.MailInsecureTLS)

	// Open the CardDAV/CalDAV store when a database path is configured. A failure
	// here disables DAV rather than preventing the whole backend from starting.
	var davStore *dav.Store
	var davAuth *davVerifier
	if cfg.DBPath != "" {
		st, err := dav.Open(cfg.DBDriver, cfg.DBPath)
		if err != nil {
			logger.Error("failed to open DAV store; DAV disabled", "error", err)
		} else {
			davStore = st
			davAuth = newDavVerifier(wm.Verify, 5*time.Minute)
		}
	}

	// Open the API-key subsystem when enabled and a database path is configured.
	// Like DAV, any failure disables the feature rather than aborting startup.
	var akStore *apikey.Store
	var akCipher *apikey.Cipher
	var akKeyLimit, akIPLimit *ratelimit.Limiter
	switch {
	case !cfg.APIKeyEnabled:
		// feature off
	case cfg.DBPath == "":
		logger.Warn("MAILFOLD_APIKEY_ENABLED is set but MAILFOLD_DB_PATH is empty; API keys disabled")
	default:
		st, err := apikey.Open(cfg.DBDriver, cfg.DBPath)
		if err != nil {
			logger.Error("failed to open API-key store; API keys disabled", "error", err)
			break
		}
		ci, err := apikey.NewCipher(cfg.APIKeyMasterKey)
		if err != nil {
			logger.Error("failed to initialise API-key cipher; API keys disabled", "error", err)
			_ = st.Close()
			break
		}
		akStore = st
		akCipher = ci
		akKeyLimit = ratelimit.New(cfg.APIKeyRateMax, cfg.APIKeyRateWindow)
		// The pre-auth IP guard counts every attempt (including failed token
		// guesses); it is looser than the per-key budget so a busy legitimate
		// integrator sharing one egress IP is not throttled.
		akIPLimit = ratelimit.New(cfg.APIKeyRateMax*20, cfg.APIKeyRateWindow)
	}

	return &Server{
		cfg:             cfg,
		mc:              mc,
		auth:            authn,
		loginLimiter:    limiter,
		metrics:         metrics.New(),
		webmail:         wm,
		webmailSessions: webmail.NewSessions(cfg.WebmailSessionTTL),
		davStore:        davStore,
		davAuth:         davAuth,
		apikeyStore:     akStore,
		apikeyCipher:    akCipher,
		apikeyKeyLimit:  akKeyLimit,
		apikeyIPLimit:   akIPLimit,
		logger:          logger,
	}
}

// GCWebmail evicts expired webmail sessions. It is intended to be called
// periodically from a background goroutine.
func (s *Server) GCWebmail() { s.webmailSessions.GC() }

// GCAPIKeys reclaims stale entries from the API-key rate limiters so their maps
// do not grow without bound (every distinct client IP and key id would otherwise
// leave a permanent entry). It is a no-op when the subsystem is disabled and is
// intended to be called on the same periodic sweep as GCWebmail.
func (s *Server) GCAPIKeys() {
	if s.apikeyKeyLimit != nil {
		s.apikeyKeyLimit.GC()
	}
	if s.apikeyIPLimit != nil {
		s.apikeyIPLimit.GC()
	}
}

// Handler builds the HTTP handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/health/ready", s.handleReady)

	// API documentation and Prometheus metrics (public).
	s.registerDocsRoutes(mux)
	mux.HandleFunc("GET /metrics", s.handleMetrics)

	// Authentication.
	s.registerAuthRoutes(mux)

	// End-user webmail (IMAP/SMTP-backed).
	s.registerWebmailRoutes(mux)
	s.registerWebmailCalendar(mux)

	// Self-hosted CardDAV/CalDAV (contacts/calendar), when configured.
	s.registerDAV(mux)

	// Machine-to-machine API keys (send/collect mail), when configured.
	s.registerAPIKeyRoutes(mux)

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
	s.registerAdminRoutes(mux)
	s.registerDomainTemplateRoutes(mux)
	s.registerMailboxTemplateRoutes(mux)
	s.registerRspamdSettingRoutes(mux)
	s.registerRateLimitMailboxRoutes(mux)
	s.registerRateLimitDomainRoutes(mux)
	s.registerPushoverRoutes(mux)
	s.registerFilterRoutes(mux)
	s.registerTempAliasRoutes(mux)

	// Static SPA (served only when a build directory is present).
	s.registerFrontend(mux)

	return s.withCommon(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "mailfold"})
}

// handleMetrics renders the collected HTTP metrics in Prometheus text format.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	s.metrics.WritePrometheus(w)
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

	// Registered as the bare "/" catch-all (no method) rather than "GET /": every
	// more specific route (the /api/* method-scoped handlers and the /dav/*
	// subtree, which handles all WebDAV verbs) is more specific and so takes
	// precedence, whereas "GET /" would conflict with the method-less /dav/
	// patterns under Go 1.22's ServeMux precedence rules.
	fileServer := http.FileServer(http.Dir(dir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean(r.URL.Path)
		if _, err := os.Stat(filepath.Join(dir, clean)); os.IsNotExist(err) && !strings.HasPrefix(r.URL.Path, "/api/") {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
