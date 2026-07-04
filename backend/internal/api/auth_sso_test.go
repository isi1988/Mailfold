package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/backend/memory"
	"github.com/emersion/go-imap/server"

	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/domainadmin"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
)

// newSSOMailcowMock combines the existing appPwMock (mint/list/revoke
// app-passwords) with a single mailbox mailcow reports as existing, so the
// callback's mailbox-matching and app-password-minting steps have something
// real to work against.
func newSSOMailcowMock(t *testing.T, mailboxUsername string) (*httptest.Server, *appPwMock) {
	t.Helper()
	appMock := &appPwMock{byName: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/get/mailbox/all", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]mailcow.Mailbox{{Username: mailboxUsername, Active: 1}})
	})
	mux.Handle("/", appMock.handler())
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, appMock
}

// newSSOTestServer builds a full Server with a database (so domain-admin
// login and SSO provider storage are available) and, when provider is
// non-nil, seeds one AllDomains SSO provider pointing at the fake OIDC
// provider — returning its id alongside the server so tests can drive
// /api/auth/sso/start?provider_id=.
func newSSOTestServer(t *testing.T, provider *fakeOIDCProvider, mailboxUsername string) (http.Handler, *Server, int64) {
	t.Helper()
	imapSrv := server.New(memory.New())
	imapSrv.AllowInsecureAuth = true
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = imapSrv.Serve(ln) }()
	t.Cleanup(func() { _ = imapSrv.Close() })

	mcURL := mockMailcow(t, 0, "").URL
	if mailboxUsername != "" {
		mcSrv, _ := newSSOMailcowMock(t, mailboxUsername)
		mcURL = mcSrv.URL
	}

	cfg := &config.Config{
		MailcowBaseURL:    mcURL,
		MailcowAPIKey:     "k",
		AdminUser:         "admin",
		AdminPassword:     "pw",
		SessionTTL:        time.Hour,
		WebmailSessionTTL: time.Hour,
		CORSOrigins:       []string{"*"},
		LoginRateMax:      1000,
		LoginRateWindow:   time.Minute,
		IMAPAddr:          ln.Addr().String(),
		MailUseTLS:        false,
	}
	// A database (and admin cipher) is only wired up when this test actually
	// needs SSO to be configured — otherwise s.sso stays nil, exactly like a
	// deployment that never set MAILFOLD_DB_PATH, so "disabled" tests exercise
	// the real disabled path rather than "configured but no such provider".
	if provider != nil || mailboxUsername != "" {
		key := make([]byte, 32)
		for i := range key {
			key[i] = byte(i + 9)
		}
		cfg.DBDriver = "sqlite"
		cfg.DBPath = t.TempDir() + "/sso.db"
		cfg.AdminEncKey = key
	}
	mc := mailcow.NewClient(cfg.MailcowBaseURL, cfg.MailcowAPIKey, false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	limiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	srv := NewServer(cfg, mc, authn, limiter, slog.New(slog.NewTextHandler(io.Discard, nil)))

	var providerID int64
	if provider != nil {
		if srv.sso == nil {
			t.Fatal("expected SSO to be configured (db + admin cipher both present)")
		}
		enc, nonce, err := srv.adminCipher.Seal([]byte("test-secret"))
		if err != nil {
			t.Fatalf("Seal: %v", err)
		}
		providerID, err = srv.domainAdminStore.CreateProvider(domainadmin.Provider{
			Name: "Test IdP", Issuer: provider.srv.URL, ClientID: "test-client",
			ClientSecretEnc: enc, ClientSecretNonce: nonce,
			RedirectURL: "https://mailfold.example/api/auth/sso/callback",
			AllDomains:  true, Active: true,
		}, time.Now())
		if err != nil {
			t.Fatalf("CreateProvider: %v", err)
		}
	}
	return srv.Handler(), srv, providerID
}

func TestSSOProvidersForDomainEndpoint(t *testing.T) {
	h, _, _ := newSSOTestServer(t, nil, "")
	rec := do(h, http.MethodGet, "/api/auth/sso/providers?domain=example.com", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("sso providers = %d", rec.Code)
	}
	var list []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Errorf("expected no providers when SSO isn't configured, got %+v", list)
	}

	provider := newFakeOIDCProvider(t)
	h2, srv2, _ := newSSOTestServer(t, provider, "")
	if srv2.sso == nil {
		t.Fatal("expected SSO to be configured against the fake provider")
	}
	rec2 := do(h2, http.MethodGet, "/api/auth/sso/providers?domain=example.com", "", "")
	_ = json.Unmarshal(rec2.Body.Bytes(), &list)
	if len(list) != 1 || list[0]["name"] != "Test IdP" {
		t.Errorf("expected the seeded AllDomains provider for any domain, got %+v", list)
	}
}

func TestSSOStartDisabled(t *testing.T) {
	h, _, _ := newSSOTestServer(t, nil, "")
	rec := do(h, http.MethodGet, "/api/auth/sso/start?provider_id=1", "", "")
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("sso start without config = %d, want 501", rec.Code)
	}
}

func TestSSOStartRedirects(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	h, _, providerID := newSSOTestServer(t, provider, "")

	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/start?provider_id="+itoa(providerID), nil)
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

// startSSOAndGetState drives GET /api/auth/sso/start?provider_id= and
// extracts the state value the way a browser following the redirect would
// carry it back.
func startSSOAndGetState(t *testing.T, h http.Handler, providerID int64) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/start?provider_id="+itoa(providerID), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	return loc.Query().Get("state")
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func TestSSOCallbackSuccess(t *testing.T) {
	const mailbox = "someone@example.com"
	provider := newFakeOIDCProvider(t)
	h, srv, providerID := newSSOTestServer(t, provider, mailbox)
	state := startSSOAndGetState(t, h, providerID)
	nonce := srv.sso.peekNonce(state)
	provider.idTokenClaims = func() map[string]any {
		return defaultClaims(provider.srv.URL, "test-client", nonce, mailbox, true)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/callback?state="+state+"&code=any-code", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("sso callback = %d, want 302", rec.Code)
	}
	// A successful SSO login mints a WEBMAIL session for the matching mailbox,
	// not an admin session.
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/#") || !strings.Contains(loc, "sso_webmail_token=") || !strings.Contains(loc, "sso_webmail_email="+url.QueryEscape(mailbox)) {
		t.Fatalf("unexpected redirect location: %q", loc)
	}

	frag := strings.TrimPrefix(loc, "/#")
	values, err := url.ParseQuery(frag)
	if err != nil {
		t.Fatalf("parse fragment: %v", err)
	}
	token := values.Get("sso_webmail_token")
	if token == "" {
		t.Fatal("expected a non-empty sso_webmail_token")
	}
	// The token is a real webmail session for the matched mailbox (verified via
	// the session store directly — the fake in-memory IMAP backend used here
	// only ever recognises one fixed "username"/"password" pair, not a minted
	// mailcow app-password, so a live IMAP round trip isn't meaningful in this
	// unit test; that IMAP-normalization path is covered separately in
	// internal/webmail's own integration tests).
	cred, ok := srv.webmailSessions.Get(token)
	if !ok || cred.Email != mailbox {
		t.Fatalf("webmail session for token = %+v, %v, want email %q", cred, ok, mailbox)
	}
	// The minted app-password was cached for reuse on the next SSO login.
	if _, ok, err := srv.domainAdminStore.GetMailboxCredential(mailbox); err != nil || !ok {
		t.Errorf("expected a cached mailbox credential after SSO login: ok=%v err=%v", ok, err)
	}
}

func TestSSOCallbackFailuresAreIndistinguishable(t *testing.T) {
	const mailbox = "someone@example.com"
	provider := newFakeOIDCProvider(t)
	h, srv, providerID := newSSOTestServer(t, provider, mailbox)

	// A provider-reported error, a bad state, and an identity that doesn't
	// resolve to any known mailbox must all redirect with exactly the same
	// generic message so a client (or attacker) cannot use the response to
	// learn which check failed.
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

	state := startSSOAndGetState(t, h, providerID)
	nonce := srv.sso.peekNonce(state)
	provider.idTokenClaims = func() map[string]any {
		return defaultClaims(provider.srv.URL, "test-client", nonce, "no-such-mailbox@example.com", true)
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
	h, _, _ := newSSOTestServer(t, nil, "")
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

// TestSSOCallbackRejectsMailboxOutsideProviderScope confirms a domain-scoped
// provider cannot be used to sign into a mailbox in a domain it isn't scoped
// to, even though the identity itself verifies correctly — the whole point of
// scoping a provider to specific domains.
func TestSSOCallbackRejectsMailboxOutsideProviderScope(t *testing.T) {
	const mailbox = "someone@other-domain.com"
	provider := newFakeOIDCProvider(t)
	h, srv, _ := newSSOTestServer(t, nil, mailbox) // nil: don't seed the AllDomains provider

	enc, nonce, err := srv.adminCipher.Seal([]byte("test-secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	providerID, err := srv.domainAdminStore.CreateProvider(domainadmin.Provider{
		Name: "Scoped", Issuer: provider.srv.URL, ClientID: "test-client",
		ClientSecretEnc: enc, ClientSecretNonce: nonce,
		RedirectURL: "https://mailfold.example/api/auth/sso/callback",
		Domains:     []string{"example.com"}, Active: true, // NOT other-domain.com
	}, time.Now())
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}

	state := startSSOAndGetState(t, h, providerID)
	oidcNonce := srv.sso.peekNonce(state)
	provider.idTokenClaims = func() map[string]any {
		return defaultClaims(provider.srv.URL, "test-client", oidcNonce, mailbox, true)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/callback?state="+state+"&code=x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || !strings.Contains(rec.Header().Get("Location"), "sso_error=") {
		t.Errorf("mailbox outside provider scope should be rejected, got code=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}
}
