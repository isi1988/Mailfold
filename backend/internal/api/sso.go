package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/isi1988/Mailfold/backend/internal/admin"
	"github.com/isi1988/Mailfold/backend/internal/domainadmin"
)

// ssoPendingTTL bounds how long an in-flight SSO redirect may take to
// complete before its state/nonce/PKCE verifier are discarded.
const ssoPendingTTL = 10 * time.Minute

// errSSOMailboxNotAllowed is returned by the callback handler (not Verify
// itself — see auth_sso.go) when the verified identity is real but does not
// resolve to a mailbox the provider is allowed to authenticate into: either
// no mailbox with that address exists, or it exists in a domain the provider
// is not scoped to. It is distinct from a verification failure so callers
// could, if useful, tell the two apart, but both are reported to the browser
// identically to avoid revealing anything about which mailboxes exist.
var errSSOMailboxNotAllowed = errors.New("identity is not authorized to sign in")

// ssoPending is the server-side state remembered between the redirect to the
// identity provider and the callback: which provider was used, the nonce
// (replay protection for the ID token), and the PKCE code verifier (so the
// authorization code is useless to anyone who intercepts it in transit
// without also having this value).
type ssoPending struct {
	providerID int64
	nonce      string
	verifier   string
	expiresAt  time.Time
}

// ssoRuntime is a discovered, ready-to-use OIDC client for one configured
// provider. Discovery is a network call, so it is cached here rather than
// repeated on every login attempt.
type ssoRuntime struct {
	oauthCfg oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// ssoManager owns OIDC single sign-on mechanics for every configured
// provider: discovery (cached per provider), the OAuth2 authorization-code
// exchange, and ID-token verification. Unlike the single-provider design it
// replaced, providers are rows in the domainadmin store (managed by the
// super-admin and by domain admins scoped to their own domains) rather than
// startup-time environment variables, so they can be added, edited, or
// removed without restarting the process. This type deliberately does not
// decide WHO a verified identity is allowed to become — that mailbox-matching
// and domain-scope decision is made by the HTTP handler in auth_sso.go, which
// also has access to the mailcow client and session stores this type does
// not need to know about.
type ssoManager struct {
	store  *domainadmin.Store
	cipher *admin.Cipher

	mu       sync.Mutex
	pending  map[string]ssoPending
	runtimes map[int64]*ssoRuntime
}

// newSSOManager wraps the domainadmin store (which persists provider
// configuration) and the shared secrets cipher. No network calls happen
// here — OIDC discovery for a given provider happens lazily, the first time
// that provider is used, and is cached thereafter.
func newSSOManager(store *domainadmin.Store, cipher *admin.Cipher) *ssoManager {
	return &ssoManager{
		store:    store,
		cipher:   cipher,
		pending:  make(map[string]ssoPending),
		runtimes: make(map[int64]*ssoRuntime),
	}
}

// invalidate discards a provider's cached discovery/runtime, so the next use
// after an edit picks up the new configuration.
func (m *ssoManager) invalidate(providerID int64) {
	m.mu.Lock()
	delete(m.runtimes, providerID)
	m.mu.Unlock()
}

// runtimeFor returns the cached OIDC runtime for providerID, performing
// discovery and decrypting the stored client secret on first use.
func (m *ssoManager) runtimeFor(ctx context.Context, providerID int64) (*ssoRuntime, error) {
	m.mu.Lock()
	rt, ok := m.runtimes[providerID]
	m.mu.Unlock()
	if ok {
		return rt, nil
	}

	p, ok, err := m.store.GetProvider(providerID)
	if err != nil {
		return nil, err
	}
	if !ok || !p.Active {
		return nil, errors.New("sso provider not found or inactive")
	}
	secret, err := m.cipher.Open(p.ClientSecretEnc, p.ClientSecretNonce)
	if err != nil {
		return nil, fmt.Errorf("decrypting client secret: %w", err)
	}
	provider, err := oidc.NewProvider(ctx, p.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery against %q: %w", p.Issuer, err)
	}
	rt = &ssoRuntime{
		oauthCfg: oauth2.Config{
			ClientID:     p.ClientID,
			ClientSecret: string(secret),
			RedirectURL:  p.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "email"},
		},
		// ClientID set (rather than SkipClientIDCheck) so the verifier enforces
		// the token's audience matches this client, and every other default
		// check (signature, issuer, expiry) stays on.
		verifier: provider.Verifier(&oidc.Config{ClientID: p.ClientID}),
	}
	m.mu.Lock()
	m.runtimes[providerID] = rt
	m.mu.Unlock()
	return rt, nil
}

// StartURL mints a fresh state/nonce/PKCE verifier for providerID, remembers
// them under the state key, and returns the URL to redirect the browser to.
func (m *ssoManager) StartURL(ctx context.Context, providerID int64) (string, error) {
	rt, err := m.runtimeFor(ctx, providerID)
	if err != nil {
		return "", err
	}
	state, err := randomHex(32)
	if err != nil {
		return "", err
	}
	nonce, err := randomHex(32)
	if err != nil {
		return "", err
	}
	verifier := oauth2.GenerateVerifier()

	m.mu.Lock()
	m.pending[state] = ssoPending{providerID: providerID, nonce: nonce, verifier: verifier, expiresAt: time.Now().Add(ssoPendingTTL)}
	m.mu.Unlock()

	return rt.oauthCfg.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), nil
}

// consumePending removes and returns the pending entry for state, if present
// and not expired. It is single-use: state is deleted on lookup regardless of
// whether it was valid, so a state value can never be redeemed twice.
func (m *ssoManager) consumePending(state string) (ssoPending, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.pending[state]
	delete(m.pending, state)
	if !ok || time.Now().After(p.expiresAt) {
		return ssoPending{}, false
	}
	return p, true
}

// GC discards expired pending flows so an abandoned or never-completed login
// attempt does not leak memory forever.
func (m *ssoManager) GC() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, p := range m.pending {
		if now.After(p.expiresAt) {
			delete(m.pending, k)
		}
	}
}

// VerifiedIdentity is the outcome of a successful callback verification: a
// real, signature-and-nonce-checked, provider-confirmed email address, together
// with which provider (and therefore which domain scope) vouched for it.
type VerifiedIdentity struct {
	Email      string
	ProviderID int64
}

// Verify completes the callback leg: it redeems the pending state (rejecting
// unknown, expired, or already-used ones), exchanges the authorization code
// for tokens using the matching PKCE verifier, verifies the ID token's
// signature/issuer/audience/expiry (via the library's defaults) and its nonce
// (which the library deliberately leaves to the caller), and requires
// email_verified. It does NOT decide whether the resulting email is allowed to
// sign in as anything — see errSSOMailboxNotAllowed and the callback handler
// in auth_sso.go for that mailbox-matching, domain-scope-enforcing step.
func (m *ssoManager) Verify(ctx context.Context, state, code string) (VerifiedIdentity, error) {
	pending, ok := m.consumePending(state)
	if !ok {
		return VerifiedIdentity{}, errors.New("sign-in expired or was already used; try again")
	}
	rt, err := m.runtimeFor(ctx, pending.providerID)
	if err != nil {
		return VerifiedIdentity{}, err
	}
	token, err := rt.oauthCfg.Exchange(ctx, code, oauth2.VerifierOption(pending.verifier))
	if err != nil {
		return VerifiedIdentity{}, fmt.Errorf("code exchange failed: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return VerifiedIdentity{}, errors.New("the identity provider did not return an ID token")
	}
	idToken, err := rt.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return VerifiedIdentity{}, fmt.Errorf("id token verification failed: %w", err)
	}
	if idToken.Nonce == "" || idToken.Nonce != pending.nonce {
		return VerifiedIdentity{}, errors.New("nonce mismatch")
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return VerifiedIdentity{}, fmt.Errorf("could not read identity claims: %w", err)
	}
	// Fail closed: a provider that omits email_verified entirely reads as
	// false here (Go's zero value), which is the safe direction for a login
	// gate — better to reject a working IdP misconfiguration loudly than to
	// accept an unverified email address as proof of identity.
	if !claims.EmailVerified {
		return VerifiedIdentity{}, errSSOMailboxNotAllowed
	}
	return VerifiedIdentity{Email: normalizeEmail(claims.Email), ProviderID: pending.providerID}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
