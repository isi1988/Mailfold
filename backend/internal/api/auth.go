package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/admin"
	"github.com/isi1988/Mailfold/backend/internal/auth"
)

// registerAuthRoutes wires the authentication endpoints onto the given mux. It
// exposes login, the two-factor verification step, and the SSO endpoints as
// public routes (a caller cannot yet have a token) while guarding logout and
// the "me" identity endpoint behind requireAuth, since both only make sense for
// an already-authenticated session. Grouping these routes in one method keeps
// the auth surface easy to audit.
func (s *Server) registerAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/2fa/verify", s.handleLogin2FAVerify)
	mux.HandleFunc("POST /api/auth/logout", s.requireAuth(s.handleLogout))
	mux.HandleFunc("GET /api/auth/me", s.requireAuth(s.handleMe))
}

// loginRequest is the request body for the login endpoint, carrying the
// credentials the caller wants to authenticate with.
type loginRequest struct {
	// User is the account name (typically the mailcow admin login) to sign in
	// as.
	User string `json:"user"`
	// Password is the plaintext password presented for verification; it is
	// checked by the auth service and never stored by this handler.
	Password string `json:"password"`
}

// handleLogin authenticates a caller's password. When the admin has not
// enrolled two-factor auth, success mints a session immediately, exactly as
// before. When they have, success instead returns a short-lived pending token
// that handleLogin2FAVerify exchanges for a session once the second factor is
// presented — the password alone is never enough to sign in. A malformed body
// yields 400, bad credentials yield 401, and any other failure yields 500.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Throttle login attempts per source IP to blunt password brute-forcing.
	// This runs before any body parsing so that flooding the endpoint is cheap
	// to reject.
	if allowed, retry := s.loginLimiter.Allow(clientIP(r)); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many login attempts, slow down"})
		return
	}

	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if !s.auth.CheckPassword(req.User, req.Password) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if s.totpEnabled() {
		pendingToken, err := s.auth.IssuePending()
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"needs_2fa": true, "pending_token": pendingToken})
		return
	}

	sess, err := s.auth.MintSession(sessionMetaFrom(r))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      sess.Token,
		"user":       sess.User,
		"expires_at": sess.ExpiresAt,
	})
}

// login2FARequest is the body for the second login step: the pending token from
// handleLogin plus either a 6-digit TOTP code or a recovery code.
type login2FARequest struct {
	PendingToken string `json:"pending_token"`
	Code         string `json:"code"`
}

// handleLogin2FAVerify redeems a pending token together with a TOTP or recovery
// code and, on success, mints the real session. A wrong code does not
// invalidate the pending token — VerifyPending only counts the attempt — so a
// typo can be retried up to auth.maxPendingAttempts times before the client
// must start over from handleLogin (which is itself rate-limited); the token
// is only consumed once a code actually verifies.
func (s *Server) handleLogin2FAVerify(w http.ResponseWriter, r *http.Request) {
	if allowed, retry := s.loginLimiter.Allow(clientIP(r)); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts, slow down"})
		return
	}

	var req login2FARequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	user, ok := s.auth.VerifyPending(req.PendingToken)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "sign in again"})
		return
	}
	if !s.verifyTOTPOrRecovery(user, req.Code) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid code"})
		return
	}
	s.auth.ConsumePending(req.PendingToken)

	sess, err := s.auth.MintSession(sessionMetaFrom(r))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      sess.Token,
		"user":       sess.User,
		"expires_at": sess.ExpiresAt,
	})
}

// totpEnabled reports whether the admin account currently has two-factor auth
// turned on. It fails closed to "disabled" if the store is unavailable, which
// keeps login working (the alternative — locking the admin out because a
// database hiccupped — is worse than a temporarily-optional second factor).
func (s *Server) totpEnabled() bool {
	if s.adminStore == nil {
		return false
	}
	acct, err := s.adminStore.GetAccount(s.cfg.AdminUser)
	if err != nil {
		s.logger.Error("failed to read admin account for 2FA check", "error", err)
		return false
	}
	return acct.TOTPEnabled
}

// verifyTOTPOrRecovery checks code against the admin's live TOTP secret first,
// then — only if that fails — against their unused recovery codes. Recovery
// codes are consumed on successful use so each one works exactly once.
func (s *Server) verifyTOTPOrRecovery(user, code string) bool {
	if s.adminStore == nil || s.adminCipher == nil {
		return false
	}
	acct, err := s.adminStore.GetAccount(user)
	if err != nil || !acct.TOTPEnabled || len(acct.TOTPSecretEnc) == 0 {
		return false
	}
	secretBytes, err := s.adminCipher.Open(acct.TOTPSecretEnc, acct.TOTPSecretNonce)
	if err == nil && admin.VerifyTOTP(string(secretBytes), code) {
		return true
	}
	ok, err := s.adminStore.ConsumeRecoveryCode(user, admin.HashRecoveryCode(code), time.Now())
	return err == nil && ok
}

// sessionMetaFrom extracts the client IP and User-Agent to record on a newly
// minted session, so the sessions list can read like a device list.
func sessionMetaFrom(r *http.Request) auth.SessionMeta {
	return auth.SessionMeta{IP: clientIP(r), UserAgent: r.UserAgent()}
}

// handleLogout invalidates the caller's session. It reads the bearer token from
// the request and asks the auth service to revoke it, then always responds with
// 200 OK so that logging out is idempotent from the client's perspective even if
// the token was already unknown or expired.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.auth.Logout(bearerToken(r))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleMe returns the authenticated caller's session. Because it runs behind
// requireAuth, the session is guaranteed to be present on the context; the
// handler simply echoes it back so a client can confirm who it is signed in as
// and when the session expires.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sessionFrom(r))
}
