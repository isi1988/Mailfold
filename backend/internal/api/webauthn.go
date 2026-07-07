package api

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/isi1988/Mailfold/backend/internal/admin"
)

// webAuthnCeremonyTTL bounds how long an in-flight registration or login
// ceremony's server-side challenge stays valid before it must be restarted.
const webAuthnCeremonyTTL = 5 * time.Minute

// openWebAuthn builds the relying-party configuration from the server's
// configured CORS origins. WebAuthn needs a single, fixed relying-party ID
// (the effective domain of the frontend); a wildcard "*" CORS config can't
// produce one, so the feature simply stays off in that case — matching every
// other optional store's fail-open behaviour. Anyone who has already locked
// CORS down to their real origin (already required for cookie-less
// bearer-token auth to work well) gets passkeys for free with no extra
// configuration.
func openWebAuthn(corsOrigins []string, logger *slog.Logger) *webauthn.WebAuthn {
	var origins []string
	for _, o := range corsOrigins {
		if o != "*" {
			origins = append(origins, o)
		}
	}
	if len(origins) == 0 {
		return nil
	}
	u, err := url.Parse(origins[0])
	if err != nil || u.Hostname() == "" {
		logger.Warn("could not derive a WebAuthn relying-party ID from MAILFOLD_CORS_ORIGINS; passkeys disabled", "origin", origins[0])
		return nil
	}
	wa, err := webauthn.New(&webauthn.Config{
		RPID:          u.Hostname(),
		RPDisplayName: "Mailfold",
		RPOrigins:     origins,
	})
	if err != nil {
		logger.Warn("failed to initialise WebAuthn; passkeys disabled", "error", err)
		return nil
	}
	return wa
}

// registerWebAuthnRoutes wires passkey/security-key enrollment (behind the
// admin session, alongside TOTP in account_totp.go) and the login-time
// verification step (public, gated on the same pending token
// /api/auth/2fa/verify uses, so a login can be completed with either a TOTP
// code or a passkey).
func (s *Server) registerWebAuthnRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/account/webauthn/credentials", s.requireAuth(s.handleWebAuthnList))
	mux.HandleFunc("POST /api/account/webauthn/register/begin", s.requireAuth(s.handleWebAuthnRegisterBegin))
	mux.HandleFunc("POST /api/account/webauthn/register/finish", s.requireAuth(s.handleWebAuthnRegisterFinish))
	mux.HandleFunc("DELETE /api/account/webauthn/credentials/{id}", s.requireAuth(s.handleWebAuthnDelete))

	mux.HandleFunc("POST /api/auth/2fa/webauthn/begin", s.handleWebAuthnLoginBegin)
	mux.HandleFunc("POST /api/auth/2fa/webauthn/finish", s.handleWebAuthnLoginFinish)
}

// requireWebAuthn reports 501 and returns false when passkeys are unavailable
// (no database, or no usable relying-party configuration — see openWebAuthn).
func (s *Server) requireWebAuthn(w http.ResponseWriter) bool {
	if s.adminStore == nil || s.webAuthn == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "set MAILFOLD_DB_PATH and a non-wildcard MAILFOLD_CORS_ORIGINS to enable passkeys"})
		return false
	}
	return true
}

// webAuthnUser adapts the admin account to the go-webauthn library's User
// interface. Mailfold has exactly one admin account, so WebAuthnID is simply
// the username; the library never displays it, it only uses it to scope
// credential lookups during a ceremony.
type webAuthnUser struct {
	username    string
	credentials []webauthn.Credential
}

func (u *webAuthnUser) WebAuthnID() []byte                         { return []byte(u.username) }
func (u *webAuthnUser) WebAuthnName() string                       { return u.username }
func (u *webAuthnUser) WebAuthnDisplayName() string                { return u.username }
func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

// loadWebAuthnUser builds a webAuthnUser from every credential stored for
// username.
func (s *Server) loadWebAuthnUser(username string) (*webAuthnUser, error) {
	stored, err := s.adminStore.ListWebAuthnCredentials(username)
	if err != nil {
		return nil, err
	}
	creds := make([]webauthn.Credential, len(stored))
	for i, c := range stored {
		creds[i] = webauthn.Credential{
			ID:            c.CredentialID,
			PublicKey:     c.PublicKey,
			Transport:     splitTransports(c.Transports),
			Authenticator: webauthn.Authenticator{SignCount: c.SignCount},
		}
	}
	return &webAuthnUser{username: username, credentials: creds}, nil
}

func splitTransports(s string) []protocol.AuthenticatorTransport {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]protocol.AuthenticatorTransport, len(parts))
	for i, p := range parts {
		out[i] = protocol.AuthenticatorTransport(p)
	}
	return out
}

func joinTransports(t []protocol.AuthenticatorTransport) string {
	parts := make([]string, len(t))
	for i, p := range t {
		parts[i] = string(p)
	}
	return strings.Join(parts, ",")
}

// webAuthnCeremonyCache holds the server-side SessionData for an in-flight
// registration or login ceremony between its Begin and Finish calls. It is
// process-local, like auth's own pending-login map, and single-use by design:
// Take deletes the entry it returns so a session can never be replayed
// against a second Finish call.
type webAuthnCeremonyCache struct {
	mu      sync.Mutex
	entries map[string]webAuthnCeremonyEntry
}

type webAuthnCeremonyEntry struct {
	session webauthn.SessionData
	expires time.Time
}

func newWebAuthnCeremonyCache() *webAuthnCeremonyCache {
	return &webAuthnCeremonyCache{entries: make(map[string]webAuthnCeremonyEntry)}
}

// Store saves session under key, expiring after webAuthnCeremonyTTL.
func (c *webAuthnCeremonyCache) Store(key string, session *webauthn.SessionData) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = webAuthnCeremonyEntry{session: *session, expires: time.Now().Add(webAuthnCeremonyTTL)}
}

// Take retrieves and removes the session stored under key, reporting whether
// it was present and not expired.
func (c *webAuthnCeremonyCache) Take(key string) (webauthn.SessionData, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	delete(c.entries, key)
	if !ok || time.Now().After(e.expires) {
		return webauthn.SessionData{}, false
	}
	return e.session, true
}

// GC discards ceremonies that were begun but never finished before expiring,
// so an abandoned enrollment or login attempt doesn't linger in memory.
func (c *webAuthnCeremonyCache) GC() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, e := range c.entries {
		if now.After(e.expires) {
			delete(c.entries, k)
		}
	}
}

// webAuthnRegisterCeremonyKey is the ceremony-cache key for the single admin
// account's in-flight registration. There is only ever one admin, so a fixed
// key (rather than one keyed by session token) is enough to prevent two
// concurrent enrollments from clobbering each other's challenge.
const webAuthnRegisterCeremonyKey = "register"

// credentialSummary is what the frontend sees for an enrolled credential —
// never the raw credential id or public key, which stay server-side.
type credentialSummary struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// handleWebAuthnList returns the admin's enrolled passkeys/security keys.
func (s *Server) handleWebAuthnList(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebAuthn(w) {
		return
	}
	creds, err := s.adminStore.ListWebAuthnCredentials(s.cfg.AdminUser)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]credentialSummary, len(creds))
	for i, c := range creds {
		out[i] = credentialSummary{ID: c.ID, Name: c.Name, CreatedAt: c.CreatedAt}
	}
	writeJSON(w, http.StatusOK, out)
}

// webAuthnRegisterBeginRequest carries the current password: starting
// enrollment is gated on it, exactly like TOTP enrollment, so a hijacked
// bearer token cannot silently plant a persistent backdoor credential without
// ever knowing the real password.
type webAuthnRegisterBeginRequest struct {
	CurrentPassword string `json:"current_password"`
}

// handleWebAuthnRegisterBegin starts an enrollment ceremony and returns the
// browser-facing creation options (challenge, relying-party info, and the
// credentials to exclude so the same authenticator can't be added twice).
func (s *Server) handleWebAuthnRegisterBegin(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebAuthn(w) {
		return
	}
	var req webAuthnRegisterBeginRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if !s.auth.CheckPassword(s.cfg.AdminUser, req.CurrentPassword) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "current password is incorrect"})
		return
	}
	user, err := s.loadWebAuthnUser(s.cfg.AdminUser)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	creation, session, err := s.webAuthn.BeginRegistration(user)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.webAuthnCeremonies.Store(webAuthnRegisterCeremonyKey, session)
	writeJSON(w, http.StatusOK, creation)
}

// handleWebAuthnRegisterFinish verifies the authenticator's response and, on
// success, stores the new credential. The request body is the raw
// CredentialCreationResponse produced by navigator.credentials.create() —
// go-webauthn parses it directly from r — so the caller-chosen label for the
// new credential travels as a query parameter instead.
func (s *Server) handleWebAuthnRegisterFinish(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebAuthn(w) {
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		name = "Passkey"
	}
	session, ok := s.webAuthnCeremonies.Take(webAuthnRegisterCeremonyKey)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "registration expired, start again"})
		return
	}
	user, err := s.loadWebAuthnUser(s.cfg.AdminUser)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	cred, err := s.webAuthn.FinishRegistration(user, session, r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "could not verify the new passkey"})
		return
	}
	err = s.adminStore.AddWebAuthnCredential(admin.WebAuthnCredential{
		Username:     s.cfg.AdminUser,
		CredentialID: cred.ID,
		PublicKey:    cred.PublicKey,
		SignCount:    cred.Authenticator.SignCount,
		Transports:   joinTransports(cred.Transport),
		Name:         name,
	}, time.Now())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleWebAuthnDelete revokes one enrolled credential by its database id.
func (s *Server) handleWebAuthnDelete(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebAuthn(w) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := s.adminStore.DeleteWebAuthnCredential(s.cfg.AdminUser, id); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// hasWebAuthnCredentials reports whether the admin account has at least one
// enrolled passkey, so handleLogin knows to require a second factor even when
// TOTP itself is off. It fails closed to "no credentials" if the store is
// unavailable — the same posture as totpEnabled.
func (s *Server) hasWebAuthnCredentials() bool {
	if s.adminStore == nil || s.webAuthn == nil {
		return false
	}
	creds, err := s.adminStore.ListWebAuthnCredentials(s.cfg.AdminUser)
	if err != nil {
		s.logger.Error("failed to read WebAuthn credentials for login check", "error", err)
		return false
	}
	return len(creds) > 0
}

// webAuthnLoginBeginRequest carries the pending token issued by handleLogin's
// password step, exactly like login2FARequest does for the TOTP path.
type webAuthnLoginBeginRequest struct {
	PendingToken string `json:"pending_token"`
}

// handleWebAuthnLoginBegin starts the login-time assertion ceremony, scoped to
// the credentials already enrolled for the pending token's user so the
// browser only offers a matching authenticator.
func (s *Server) handleWebAuthnLoginBegin(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebAuthn(w) {
		return
	}
	var req webAuthnLoginBeginRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	username, ok := s.auth.VerifyPending(req.PendingToken)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": errSignInAgain})
		return
	}
	user, err := s.loadWebAuthnUser(username)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(user.credentials) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no passkey enrolled"})
		return
	}
	assertion, session, err := s.webAuthn.BeginLogin(user)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.webAuthnCeremonies.Store(req.PendingToken, session)
	writeJSON(w, http.StatusOK, assertion)
}

// handleWebAuthnLoginFinish verifies the authenticator's assertion and, on
// success, redeems the pending token for a real session — mirroring
// handleLogin2FAVerify's success path exactly (audit log, failure-streak
// reset, new-device alert) so a passkey login is indistinguishable from a
// TOTP login to every downstream feature. The assertion response is the raw
// body go-webauthn parses from r directly, so the pending token travels as a
// query parameter.
func (s *Server) handleWebAuthnLoginFinish(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebAuthn(w) {
		return
	}
	pendingToken := r.URL.Query().Get("pending_token")
	username, ok := s.auth.VerifyPending(pendingToken)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": errSignInAgain})
		return
	}
	session, ok := s.webAuthnCeremonies.Take(pendingToken)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": errSignInAgain})
		return
	}
	user, err := s.loadWebAuthnUser(username)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	cred, err := s.webAuthn.FinishLogin(user, session, r)
	if err != nil {
		s.logger.Warn("passkey login verification failed", "user", username, "error", err)
		s.recordAudit("admin", username, "login_failed", http.StatusUnauthorized, clientIP(r))
		s.alertOnFailedLogin(username, r)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "passkey verification failed"})
		return
	}
	if err := s.adminStore.UpdateWebAuthnSignCount(cred.ID, cred.Authenticator.SignCount); err != nil {
		s.logger.Error("failed to update WebAuthn sign count", "error", err)
	}
	s.auth.ConsumePending(pendingToken)

	sess, err := s.auth.MintSession(sessionMetaFrom(r))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.recordAudit("admin", sess.User, "login", http.StatusOK, clientIP(r))
	s.loginFailures.Reset(sess.User)
	s.alertOnNewDevice(sess.User, r)
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      sess.Token,
		"user":       sess.User,
		"expires_at": sess.ExpiresAt,
	})
}
