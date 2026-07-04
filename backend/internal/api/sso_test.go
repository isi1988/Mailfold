package api

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	josejwt "github.com/go-jose/go-jose/v4"

	"github.com/isi1988/Mailfold/backend/internal/config"
)

// fakeOIDCProvider is a minimal OIDC identity provider for tests: it serves
// discovery, a JWKS with one RSA key, and a token endpoint whose ID token
// claims are supplied per-test via idTokenClaims.
type fakeOIDCProvider struct {
	srv        *httptest.Server
	privateKey *rsa.PrivateKey
	kid        string

	// idTokenClaims builds the claims for the next issued ID token. Tests set
	// this to control exactly what the "identity provider" asserts; it closes
	// over whatever nonce the test already read back via peekNonce, since the
	// real token request carries no such field for the fake to echo.
	idTokenClaims func() map[string]any
	// tokenStatus/tokenBody let a test simulate the token endpoint failing
	// outright (e.g. a rejected authorization code) instead of returning a
	// well-formed token response.
	tokenStatus int
}

func newFakeOIDCProvider(t *testing.T) *fakeOIDCProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	p := &fakeOIDCProvider{privateKey: key, kid: "test-key-1", tokenStatus: http.StatusOK}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                p.srv.URL,
			"authorization_endpoint":                p.srv.URL + "/auth",
			"token_endpoint":                        p.srv.URL + "/token",
			"jwks_uri":                              p.srv.URL + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {
		jwk := josejwt.JSONWebKey{Key: &p.privateKey.PublicKey, KeyID: p.kid, Algorithm: "RS256", Use: "sig"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(josejwt.JSONWebKeySet{Keys: []josejwt.JSONWebKey{jwk}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if p.tokenStatus != http.StatusOK {
			w.WriteHeader(p.tokenStatus)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		idToken := p.signIDToken(t, p.idTokenClaims())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-access-token",
			"token_type":   "Bearer",
			"id_token":     idToken,
		})
	})

	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	return p
}

// signIDToken builds and signs a minimal JWS carrying claims as the payload.
func (p *fakeOIDCProvider) signIDToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	signer, err := josejwt.NewSigner(
		josejwt.SigningKey{Algorithm: josejwt.RS256, Key: p.privateKey},
		(&josejwt.SignerOptions{}).WithType("JWT").WithHeader("kid", p.kid),
	)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	jws, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	compact, err := jws.CompactSerialize()
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	return compact
}

// defaultClaims returns a valid, fully-authorized claim set for email,
// overridable per test.
func defaultClaims(issuer, clientID, nonce, email string, emailVerified bool) map[string]any {
	now := time.Now()
	return map[string]any{
		"iss":            issuer,
		"sub":            "user-123",
		"aud":            clientID,
		"exp":            now.Add(time.Hour).Unix(),
		"iat":            now.Unix(),
		"nonce":          nonce,
		"email":          email,
		"email_verified": emailVerified,
	}
}

const testAllowedEmail = "admin@example.com"

func newTestSSOManager(t *testing.T, provider *fakeOIDCProvider) *ssoManager {
	t.Helper()
	cfg := &config.Config{
		OIDCIssuer:       provider.srv.URL,
		OIDCClientID:     "test-client",
		OIDCClientSecret: "test-secret",
		OIDCRedirectURL:  "https://mailfold.example/api/auth/sso/callback",
		OIDCAllowedEmail: testAllowedEmail,
	}
	mgr, err := newSSOManager(context.Background(), cfg)
	if err != nil {
		t.Fatalf("newSSOManager: %v", err)
	}
	return mgr
}

// startAndExtractState mints a StartURL and pulls the "state" query parameter
// back out of it, the way a browser redirect would carry it to the callback.
func startAndExtractState(t *testing.T, mgr *ssoManager) string {
	t.Helper()
	dest, err := mgr.StartURL()
	if err != nil {
		t.Fatalf("StartURL: %v", err)
	}
	u, err := url.Parse(dest)
	if err != nil {
		t.Fatalf("parse StartURL: %v", err)
	}
	state := u.Query().Get("state")
	if state == "" {
		t.Fatal("StartURL did not include a state parameter")
	}
	return state
}

// peekNonce reads back the nonce StartURL generated for state, since a test
// cannot know it ahead of time (it's minted inside StartURL) but needs it to
// build a matching ID token via the fake provider's idTokenClaims closure.
func (m *ssoManager) peekNonce(state string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pending[state].nonce
}

func TestSSOManagerFullFlowSuccess(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	mgr := newTestSSOManager(t, provider)
	state := startAndExtractState(t, mgr)
	nonce := mgr.peekNonce(state)

	provider.idTokenClaims = func() map[string]any {
		return defaultClaims(provider.srv.URL, "test-client", nonce, testAllowedEmail, true)
	}

	email, err := mgr.Verify(context.Background(), state, "any-code")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if email != testAllowedEmail {
		t.Errorf("Verify email = %q, want %q", email, testAllowedEmail)
	}
}

func TestSSOManagerUnknownState(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	mgr := newTestSSOManager(t, provider)
	if _, err := mgr.Verify(context.Background(), "not-a-real-state", "code"); err == nil {
		t.Error("Verify should reject an unknown state")
	}
}

func TestSSOManagerStateIsSingleUse(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	mgr := newTestSSOManager(t, provider)
	state := startAndExtractState(t, mgr)
	nonce := mgr.peekNonce(state)
	provider.idTokenClaims = func() map[string]any {
		return defaultClaims(provider.srv.URL, "test-client", nonce, testAllowedEmail, true)
	}

	if _, err := mgr.Verify(context.Background(), state, "code"); err != nil {
		t.Fatalf("first Verify: %v", err)
	}
	if _, err := mgr.Verify(context.Background(), state, "code"); err == nil {
		t.Error("a state must not be redeemable twice")
	}
}

func TestSSOManagerExpiredState(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	mgr := newTestSSOManager(t, provider)
	state := startAndExtractState(t, mgr)

	mgr.mu.Lock()
	p := mgr.pending[state]
	p.expiresAt = time.Now().Add(-time.Minute)
	mgr.pending[state] = p
	mgr.mu.Unlock()

	if _, err := mgr.Verify(context.Background(), state, "code"); err == nil {
		t.Error("Verify should reject an expired state")
	}
}

func TestSSOManagerNonceMismatch(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	mgr := newTestSSOManager(t, provider)
	state := startAndExtractState(t, mgr)

	provider.idTokenClaims = func() map[string]any {
		return defaultClaims(provider.srv.URL, "test-client", "wrong-nonce", testAllowedEmail, true)
	}
	if _, err := mgr.Verify(context.Background(), state, "code"); err == nil {
		t.Error("Verify should reject a mismatched nonce")
	}
}

func TestSSOManagerEmailNotVerified(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	mgr := newTestSSOManager(t, provider)
	state := startAndExtractState(t, mgr)
	nonce := mgr.peekNonce(state)

	provider.idTokenClaims = func() map[string]any {
		return defaultClaims(provider.srv.URL, "test-client", nonce, testAllowedEmail, false)
	}
	_, err := mgr.Verify(context.Background(), state, "code")
	if err != errSSONotAuthorized {
		t.Errorf("want errSSONotAuthorized for an unverified email, got %v", err)
	}
}

func TestSSOManagerWrongEmail(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	mgr := newTestSSOManager(t, provider)
	state := startAndExtractState(t, mgr)
	nonce := mgr.peekNonce(state)

	provider.idTokenClaims = func() map[string]any {
		return defaultClaims(provider.srv.URL, "test-client", nonce, "someone-else@example.com", true)
	}
	_, err := mgr.Verify(context.Background(), state, "code")
	if err != errSSONotAuthorized {
		t.Errorf("want errSSONotAuthorized for a mismatched email, got %v", err)
	}
}

func TestSSOManagerEmailCaseInsensitive(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	mgr := newTestSSOManager(t, provider)
	state := startAndExtractState(t, mgr)
	nonce := mgr.peekNonce(state)

	provider.idTokenClaims = func() map[string]any {
		return defaultClaims(provider.srv.URL, "test-client", nonce, strings.ToUpper(testAllowedEmail), true)
	}
	email, err := mgr.Verify(context.Background(), state, "code")
	if err != nil {
		t.Fatalf("Verify should accept a case-different match: %v", err)
	}
	if email != strings.ToUpper(testAllowedEmail) {
		t.Errorf("Verify returned %q", email)
	}
}

func TestSSOManagerTokenEndpointFailure(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	provider.tokenStatus = http.StatusBadRequest
	mgr := newTestSSOManager(t, provider)
	state := startAndExtractState(t, mgr)

	if _, err := mgr.Verify(context.Background(), state, "bad-code"); err == nil {
		t.Error("Verify should surface a token-endpoint failure as an error")
	}
}

func TestSSOManagerWrongAudience(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	mgr := newTestSSOManager(t, provider)
	state := startAndExtractState(t, mgr)
	nonce := mgr.peekNonce(state)

	provider.idTokenClaims = func() map[string]any {
		return defaultClaims(provider.srv.URL, "some-other-client", nonce, testAllowedEmail, true)
	}
	if _, err := mgr.Verify(context.Background(), state, "code"); err == nil {
		t.Error("Verify should reject an ID token issued for a different client (audience)")
	}
}

func TestSSOManagerExpiredToken(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	mgr := newTestSSOManager(t, provider)
	state := startAndExtractState(t, mgr)
	nonce := mgr.peekNonce(state)

	provider.idTokenClaims = func() map[string]any {
		c := defaultClaims(provider.srv.URL, "test-client", nonce, testAllowedEmail, true)
		c["exp"] = time.Now().Add(-time.Hour).Unix()
		return c
	}
	if _, err := mgr.Verify(context.Background(), state, "code"); err == nil {
		t.Error("Verify should reject an expired ID token")
	}
}

func TestNewSSOManagerDiscoveryFailure(t *testing.T) {
	cfg := &config.Config{
		OIDCIssuer:       "http://127.0.0.1:1", // nothing listens here
		OIDCClientID:     "x",
		OIDCClientSecret: "y",
		OIDCRedirectURL:  "https://mailfold.example/callback",
		OIDCAllowedEmail: "a@example.com",
	}
	if _, err := newSSOManager(context.Background(), cfg); err == nil {
		t.Error("newSSOManager should fail when discovery is unreachable")
	}
}

func TestSSOManagerGC(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	mgr := newTestSSOManager(t, provider)
	state := startAndExtractState(t, mgr)

	mgr.mu.Lock()
	p := mgr.pending[state]
	p.expiresAt = time.Now().Add(-time.Minute)
	mgr.pending[state] = p
	n := len(mgr.pending)
	mgr.mu.Unlock()
	if n != 1 {
		t.Fatalf("want 1 pending entry before GC, got %d", n)
	}

	mgr.GC()

	mgr.mu.Lock()
	n = len(mgr.pending)
	mgr.mu.Unlock()
	if n != 0 {
		t.Errorf("GC should have removed the expired entry, %d remain", n)
	}
}
