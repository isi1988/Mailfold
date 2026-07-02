package api

import (
	"errors"
	"net/http"

	"github.com/isi1988/Mailfold/backend/internal/auth"
)

// registerAuthRoutes wires the authentication endpoints onto the given mux. It
// exposes login as a public route (a caller cannot yet have a token) while
// guarding logout and the "me" identity endpoint behind requireAuth, since both
// only make sense for an already-authenticated session. Grouping these routes in
// one method keeps the auth surface easy to audit.
func (s *Server) registerAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
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

// handleLogin authenticates a caller and issues a session token. It decodes the
// credentials, delegates verification to the auth service, and on success
// returns the token together with the user and expiry so the client can store
// the token and know when it must re-authenticate. A malformed body yields 400,
// bad credentials yield 401 (distinguished so the client can prompt for a retry
// rather than treat it as a server fault), and any other failure yields 500.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	sess, err := s.auth.Login(req.User, req.Password)
	if err != nil {
		// A credential mismatch is an expected client-side condition, so it is
		// reported as 401 with a clear message rather than logged as a server
		// error; every other error is unexpected and surfaced as a 500.
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
			return
		}
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      sess.Token,
		"user":       sess.User,
		"expires_at": sess.ExpiresAt,
	})
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
