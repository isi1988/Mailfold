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

	"github.com/isi1988/Mailfold/backend/internal/admin"
	"github.com/isi1988/Mailfold/backend/internal/apikey"
	"github.com/isi1988/Mailfold/backend/internal/audit"
	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/dav"
	"github.com/isi1988/Mailfold/backend/internal/domainadmin"
	"github.com/isi1988/Mailfold/backend/internal/metrics"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
	"github.com/isi1988/Mailfold/backend/internal/webmail"
	"github.com/isi1988/Mailfold/backend/internal/webmailuser"
)

// domainAdminSessionTTL bounds how long a domain admin's Mailfold session
// (distinct from both the singleton super-admin's and a webmail user's) stays
// valid before they must sign in again.
const domainAdminSessionTTL = 12 * time.Hour

// webmailPending2FATTL bounds how long a webmail login that passed the
// password check but still needs a TOTP/recovery code stays redeemable.
const webmailPending2FATTL = 5 * time.Minute

// Server wires HTTP routes to the mailcow API and the authenticator. It holds
// its collaborators through interfaces/values so the transport layer stays
// decoupled from concrete implementations.
type Server struct {
	cfg                 *config.Config
	mc                  Mailcow
	auth                *auth.Authenticator
	loginLimiter        *ratelimit.Limiter
	metrics             *metrics.Metrics
	webmail             *webmail.Client
	webmailSessions     *webmail.Sessions
	webmailPending      *webmail.Sessions  // holds {email,password} between a webmail login's password step and its 2FA step
	webmailUsers        *webmailuser.Store // signature/2FA per mailbox; nil when DBPath is empty
	davStore            *dav.Store
	davAuth             *davVerifier
	apikeyStore         *apikey.Store
	apikeyCipher        *apikey.Cipher
	apikeyKeyLimit      *ratelimit.Limiter    // per-key request budget
	apikeyIPLimit       *ratelimit.Limiter    // pre-auth per-IP guard against token guessing
	adminStore          *admin.Store          // password/profile/2FA/notify-sender/reset-tokens; nil when DBPath is empty
	adminCipher         *admin.Cipher         // nil when MAILFOLD_ADMIN_ENC_KEY is unset (2FA/notify-sender then report 501)
	resetLimiter        *ratelimit.Limiter    // throttles the public forgot-password endpoint per IP
	domainAdminStore    *domainadmin.Store    // domain-admin login + SSO provider config; nil when DBPath is empty
	domainAdminSessions *domainadmin.Sessions // domain-admin Mailfold sessions (distinct from the super-admin's and webmail's)
	sso                 *ssoManager           // nil unless a database and the admin cipher are both available
	auditStore          *audit.Store          // records admin/domain-admin logins and mutating actions; nil when DBPath is empty
	loginFailures       *loginFailureTracker  // consecutive-failure streaks for the admin login-alert email
	logger              *slog.Logger
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

	// Open the admin-account store whenever a database is configured — it backs
	// password change, profile, sessions and (when MAILFOLD_ADMIN_ENC_KEY is
	// also set) two-factor auth and the notification sender.
	adminStore, adminCipher := openAdminStore(cfg, authn, logger)

	// Open the webmail-user store (signature, 2FA) alongside the admin store;
	// like the other optional stores, a failure disables just this feature.
	webmailUsers := openWebmailUserStore(cfg, logger)

	// Open the domain-admin store (login + SSO provider config) alongside the
	// others; SSO itself additionally needs the admin cipher to decrypt stored
	// client secrets, so it stays nil until both are available.
	domainAdminStore := openDomainAdminStore(cfg, logger)
	var sso *ssoManager
	if domainAdminStore != nil && adminCipher != nil {
		sso = newSSOManager(domainAdminStore, adminCipher)
	}

	// Open the audit-log store alongside the others; a failure disables just
	// the audit trail rather than the whole backend.
	auditStore := openAuditStore(cfg, logger)

	return &Server{
		cfg:                 cfg,
		mc:                  mc,
		auth:                authn,
		loginLimiter:        limiter,
		metrics:             metrics.New(),
		webmail:             wm,
		webmailSessions:     webmail.NewSessions(cfg.WebmailSessionTTL),
		webmailPending:      webmail.NewSessions(webmailPending2FATTL),
		webmailUsers:        webmailUsers,
		davStore:            davStore,
		davAuth:             davAuth,
		apikeyStore:         akStore,
		apikeyCipher:        akCipher,
		apikeyKeyLimit:      akKeyLimit,
		apikeyIPLimit:       akIPLimit,
		adminStore:          adminStore,
		adminCipher:         adminCipher,
		resetLimiter:        ratelimit.New(5, time.Hour),
		domainAdminStore:    domainAdminStore,
		domainAdminSessions: domainadmin.NewSessions(domainAdminSessionTTL),
		sso:                 sso,
		auditStore:          auditStore,
		loginFailures:       newLoginFailureTracker(),
		logger:              logger,
	}
}

// openAdminStore opens the admin-account store when a database is configured
// and, on success, loads any previously-saved password-hash override into authn
// so a password changed in an earlier run takes effect immediately on this one,
// and initialises the admin cipher when MAILFOLD_ADMIN_ENC_KEY is set. Any
// failure along the way disables just the affected feature rather than
// preventing the whole backend from starting, matching the DAV/API-key stores'
// behaviour above.
func openAdminStore(cfg *config.Config, authn *auth.Authenticator, logger *slog.Logger) (*admin.Store, *admin.Cipher) {
	if cfg.DBPath == "" {
		return nil, nil
	}
	st, err := admin.Open(cfg.DBDriver, cfg.DBPath)
	if err != nil {
		logger.Error("failed to open admin-account store; password change/2FA/profile disabled", "error", err)
		return nil, nil
	}
	if acct, err := st.GetAccount(cfg.AdminUser); err == nil && acct.PasswordHash != "" {
		authn.SetPasswordHash(acct.PasswordHash)
	}
	var adminCipher *admin.Cipher
	if len(cfg.AdminEncKey) > 0 {
		if ci, err := admin.NewCipher(cfg.AdminEncKey); err == nil {
			adminCipher = ci
		} else {
			logger.Error("failed to initialise admin cipher; 2FA/notify-sender disabled", "error", err)
		}
	}
	return st, adminCipher
}

// openWebmailUserStore opens the webmail-user store (mailbox signature and
// optional 2FA enrollment) when a database is configured, matching the
// admin/DAV/API-key stores' fail-open behaviour.
func openWebmailUserStore(cfg *config.Config, logger *slog.Logger) *webmailuser.Store {
	if cfg.DBPath == "" {
		return nil
	}
	st, err := webmailuser.Open(cfg.DBDriver, cfg.DBPath)
	if err != nil {
		logger.Error("failed to open webmail-user store; signature/2FA disabled", "error", err)
		return nil
	}
	return st
}

// openDomainAdminStore opens the domain-admin store (login credentials and
// SSO provider configuration) when a database is configured, matching the
// admin/DAV/API-key/webmail-user stores' fail-open behaviour.
func openDomainAdminStore(cfg *config.Config, logger *slog.Logger) *domainadmin.Store {
	if cfg.DBPath == "" {
		return nil
	}
	st, err := domainadmin.Open(cfg.DBDriver, cfg.DBPath)
	if err != nil {
		logger.Error("failed to open domain-admin store; domain-admin login and SSO disabled", "error", err)
		return nil
	}
	return st
}

// openAuditStore opens the audit-log store when a database is configured,
// matching the other stores' fail-open behaviour: without it, the admin panel
// and domain-admin logins/actions simply go unrecorded rather than the whole
// backend failing to start.
func openAuditStore(cfg *config.Config, logger *slog.Logger) *audit.Store {
	if cfg.DBPath == "" {
		return nil
	}
	st, err := audit.Open(cfg.DBDriver, cfg.DBPath)
	if err != nil {
		logger.Error("failed to open audit-log store; audit logging disabled", "error", err)
		return nil
	}
	return st
}

// GCWebmail evicts expired webmail sessions and pending 2FA logins. It is
// intended to be called periodically from a background goroutine.
func (s *Server) GCWebmail() {
	s.webmailSessions.GC()
	s.webmailPending.GC()
}

// GCSSO discards expired in-flight SSO login attempts and expired domain-admin
// sessions. It is intended to be called on the same periodic sweep as
// GCWebmail.
func (s *Server) GCSSO() {
	if s.sso != nil {
		s.sso.GC()
	}
	s.domainAdminSessions.GC()
}

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
	s.registerPasswordResetRoutes(mux)
	s.registerSSORoutes(mux)
	s.registerDomainAdminAuthRoutes(mux)
	s.registerSSOProviderRoutes(mux)
	s.registerDomainAdminSSORoutes(mux)

	// Admin account settings: profile, password, sessions, two-factor auth,
	// and the notification sender used to email reset links.
	s.registerAccountRoutes(mux)
	s.registerTOTPRoutes(mux)
	s.registerNotifySenderRoutes(mux)

	// End-user webmail (IMAP/SMTP-backed).
	s.registerWebmailRoutes(mux)
	s.registerWebmailCalendar(mux)
	s.registerWebmailSignatureRoutes(mux)
	s.registerWebmailRuleRoutes(mux)
	s.registerWebmailTOTPRoutes(mux)

	// Self-hosted CardDAV/CalDAV (contacts/calendar), when configured.
	s.registerDAV(mux)

	// Machine-to-machine API keys (send/collect mail), when configured.
	s.registerAPIKeyRoutes(mux)
	s.registerDeviceLoginRoutes(mux)

	// Management resources (all require authentication).
	s.registerAuditLogRoutes(mux)
	s.registerStatusRoutes(mux)
	s.registerDomainRoutes(mux)
	s.registerDomainDNSRoutes(mux)
	s.registerMailboxRoutes(mux)
	s.registerMailboxBulkRoutes(mux)
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
