package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
)

// newSSOTestServer builds a full Server with SSO wired to a fake OIDC
// provider (when provider is non-nil) so the HTTP surface can be exercised
// end to end, including the session minted after a successful callback.
func newSSOTestServer(t *testing.T, provider *fakeOIDCProvider) (http.Handler, *Server) {
	t.Helper()
	cfg := &config.Config{
		MailcowBaseURL:  mockMailcow(t, 0, "").URL,
		MailcowAPIKey:   "k",
		AdminUser:       "admin",
		AdminPassword:   "pw",
		SessionTTL:      time.Hour,
		CORSOrigins:     []string{"*"},
		LoginRateMax:    1000,
		LoginRateWindow: time.Minute,
	}
	if provider != nil {
		cfg.OIDCIssuer = provider.srv.URL
		cfg.OIDCClientID = "test-client"
		cfg.OIDCClientSecret = "test-secret"
		cfg.OIDCRedirectURL = "https://mailfold.example/api/auth/sso/callback"
		cfg.OIDCAllowedEmail = testAllowedEmail
	}
	mc := mailcow.NewClient(cfg.MailcowBaseURL, cfg.MailcowAPIKey, false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	limiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	srv := NewServer(cfg, mc, authn, limiter, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return srv.Handler(), srv
}

func TestSSOConfigEndpoint(t *testing.T) {
	h, _ := newSSOTestServer(t, nil)
	rec := do(h, http.MethodGet, "/api/auth/sso/config", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("sso config = %d", rec.Code)
	}
	var body map[string]bool
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["enabled"] {
		t.Error("sso should report disabled when unconfigured")
	}

	provider := newFakeOIDCProvider(t)
	h2, srv2 := newSSOTestServer(t, provider)
	if srv2.sso == nil {
		t.Fatal("expected SSO to be configured against the fake provider")
	}
	rec2 := do(h2, http.MethodGet, "/api/auth/sso/config", "", "")
	_ = json.Unmarshal(rec2.Body.Bytes(), &body)
	if !body["enabled"] {
		t.Error("sso should report enabled when fully configured")
	}
}

func TestSSOStartDisabled(t *testing.T) {
	h, _ := newSSOTestServer(t, nil)
	rec := do(h, http.MethodGet, "/api/auth/sso/start", "", "")
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("sso start without config = %d, want 501", rec.Code)
	}
}

func TestSSOStartRedirects(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	h, _ := newSSOTestServer(t, provider)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/start", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("sso start = %d, want 302", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	q := loc.Query()
	if q.Get("client_id") != "test-client" {
		t.Errorf("client_id = %q", q.Get("client_id"))
	}
	if q.Get("state") == "" {
		t.Error("expected a state parameter")
	}
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		t.Errorf("expected PKCE parameters, got challenge=%q method=%q", q.Get("code_challenge"), q.Get("code_challenge_method"))
	}
	if q.Get("nonce") == "" {
		t.Error("expected a nonce parameter")
	}
}

// startSSOAndGetState drives GET /api/auth/sso/start and extracts the state
// value the way a browser following the redirect would carry it back.
func startSSOAndGetState(t *testing.T, h http.Handler) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/start", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	return loc.Query().Get("state")
}

func TestSSOCallbackSuccess(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	h, srv := newSSOTestServer(t, provider)
	state := startSSOAndGetState(t, h)
	nonce := srv.sso.peekNonce(state)
	provider.idTokenClaims = func() map[string]any {
		return defaultClaims(provider.srv.URL, "test-client", nonce, testAllowedEmail, true)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/callback?state="+state+"&code=any-code", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("sso callback = %d, want 302", rec.Code)
	}
	// sso_user is the Mailfold admin username the session belongs to (there is
	// only ever one), not the OIDC identity's email — the same as any other
	// login path.
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/#") || !strings.Contains(loc, "sso_token=") || !strings.Contains(loc, "sso_user=admin") {
		t.Fatalf("unexpected redirect location: %q", loc)
	}

	// The minted token should actually authenticate as the admin user.
	frag := strings.TrimPrefix(loc, "/#")
	values, err := url.ParseQuery(frag)
	if err != nil {
		t.Fatalf("parse fragment: %v", err)
	}
	token := values.Get("sso_token")
	if token == "" {
		t.Fatal("expected a non-empty sso_token")
	}
	if rec2 := do(h, http.MethodGet, "/api/auth/me", token, ""); rec2.Code != http.StatusOK {
		t.Errorf("session from SSO should authenticate, got %d", rec2.Code)
	}
}

func TestSSOCallbackFailuresAreIndistinguishable(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	h, srv := newSSOTestServer(t, provider)

	// A provider-reported error, a bad state, and a wrong email must all
	// redirect with exactly the same generic message so a client (or attacker)
	// cannot use the response to learn which check failed.
	extractError := func(rec *httptest.ResponseRecorder) string {
		loc := rec.Header().Get("Location")
		frag := strings.TrimPrefix(loc, "/#")
		values, _ := url.ParseQuery(frag)
		return values.Get("sso_error")
	}

	req1 := httptest.NewRequest(http.MethodGet, "/api/auth/sso/callback?error=access_denied", nil)
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/api/auth/sso/callback?state=bogus&code=x", nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	state := startSSOAndGetState(t, h)
	nonce := srv.sso.peekNonce(state)
	provider.idTokenClaims = func() map[string]any {
		return defaultClaims(provider.srv.URL, "test-client", nonce, "not-the-admin@example.com", true)
	}
	req3 := httptest.NewRequest(http.MethodGet, "/api/auth/sso/callback?state="+state+"&code=x", nil)
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, req3)

	for i, rec := range []*httptest.ResponseRecorder{rec1, rec2, rec3} {
		if rec.Code != http.StatusFound {
			t.Errorf("case %d: code = %d, want 302", i+1, rec.Code)
		}
	}
	e1, e2, e3 := extractError(rec1), extractError(rec2), extractError(rec3)
	if e1 == "" || e1 != e2 || e2 != e3 {
		t.Errorf("expected identical generic errors, got %q / %q / %q", e1, e2, e3)
	}
}

func TestSSOCallbackDisabled(t *testing.T) {
	h, _ := newSSOTestServer(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/callback?state=x&code=y", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("sso callback disabled = %d, want 302", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "sso_error=") {
		t.Errorf("expected an sso_error redirect, got %q", rec.Header().Get("Location"))
	}
}
