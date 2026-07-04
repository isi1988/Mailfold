package api

import (
	"net/http"
	"net/url"
)

// registerSSORoutes wires the OIDC single sign-on endpoints. All three are
// public (a caller cannot have a Mailfold token before completing this flow):
// config lets the frontend feature-detect whether to show an SSO button at
// all, start begins the redirect to the identity provider, and callback
// completes it. Every route reports the feature as simply unavailable when
// s.sso is nil (SSO was not configured, or discovery failed at startup) —
// there is nothing sensitive to hide about that.
func (s *Server) registerSSORoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/sso/config", s.handleSSOConfig)
	mux.HandleFunc("GET /api/auth/sso/start", s.handleSSOStart)
	mux.HandleFunc("GET /api/auth/sso/callback", s.handleSSOCallback)
}

// handleSSOConfig lets the frontend decide whether to show an SSO option at
// all, without exposing the issuer or client configuration.
func (s *Server) handleSSOConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": s.sso != nil})
}

// handleSSOStart redirects the browser to the identity provider to begin the
// authorization-code flow.
func (s *Server) handleSSOStart(w http.ResponseWriter, r *http.Request) {
	if s.sso == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "SSO is not configured"})
		return
	}
	dest, err := s.sso.StartURL()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

// ssoGenericFailureMsg is shown for every security-relevant callback failure
// (provider error, expired/reused state, bad token, mismatched identity) so
// the browser — and anyone watching over the user's shoulder — never learns
// which check failed. Only "SSO is not configured" (a deployment issue, not a
// security-sensitive one) is reported distinctly.
const ssoGenericFailureMsg = "sign-in failed"

// ssoFrontendRedirect sends the browser back to the SPA's root with the
// outcome encoded in the URL fragment rather than the query string: fragments
// are never sent to the server (no access/error logs) or included in the
// Referer header on any subsequent navigation, unlike a query parameter.
func ssoFrontendRedirect(w http.ResponseWriter, r *http.Request, fragment string) {
	http.Redirect(w, r, "/#"+fragment, http.StatusFound)
}

// ssoErrorRedirect is the failure-path shorthand: it always uses the
// "sso_error" fragment key so the frontend has exactly one field to check.
func ssoErrorRedirect(w http.ResponseWriter, r *http.Request, msg string) {
	ssoFrontendRedirect(w, r, "sso_error="+url.QueryEscape(msg))
}

// handleSSOCallback completes the authorization-code exchange and, on a fully
// verified and authorized identity, mints a real Mailfold session exactly the
// way the password+2FA login path does. Every failure — a provider-reported
// error, an expired/reused state, a bad token, or a mismatched identity —
// redirects with the same generic message; the specific reason is logged
// server-side only, so the browser (and anyone watching over the user's
// shoulder) never learns which check failed.
func (s *Server) handleSSOCallback(w http.ResponseWriter, r *http.Request) {
	if s.sso == nil {
		ssoErrorRedirect(w, r, "SSO is not configured")
		return
	}
	q := r.URL.Query()
	if errParam := q.Get("error"); errParam != "" {
		s.logger.Warn("sso callback: provider reported an error", "error", errParam)
		ssoErrorRedirect(w, r, ssoGenericFailureMsg)
		return
	}

	email, err := s.sso.Verify(r.Context(), q.Get("state"), q.Get("code"))
	if err != nil {
		s.logger.Warn("sso callback failed", "error", err)
		ssoErrorRedirect(w, r, ssoGenericFailureMsg)
		return
	}

	sess, err := s.auth.MintSession(sessionMetaFrom(r))
	if err != nil {
		s.logger.Error("sso: failed to mint session", "error", err, "email", email)
		ssoErrorRedirect(w, r, ssoGenericFailureMsg)
		return
	}
	ssoFrontendRedirect(w, r, "sso_token="+url.QueryEscape(sess.Token)+"&sso_user="+url.QueryEscape(sess.User))
}
