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

	"github.com/isi1988/Mailfold/backend/internal/config"
)

// ssoPendingTTL bounds how long an in-flight SSO redirect may take to
// complete before its state/nonce/PKCE verifier are discarded.
const ssoPendingTTL = 10 * time.Minute

// errSSONotAuthorized is returned by Verify when the token is valid — a real,
// signature-checked identity from the configured provider — but does not match
// the single configured allowed identity. It is distinct from a malformed or
// forged token so callers could, if ever useful, tell the two apart; today
// both are reported to the browser identically to avoid revealing anything
// about the allowed identity.
var errSSONotAuthorized = errors.New("identity is not authorized to sign in")

// ssoPending is the server-side state remembered between the redirect to the
// identity provider and the callback: the nonce (replay protection for the ID
// token) and the PKCE code verifier (so the authorization code is useless to
// anyone who intercepts it in transit without also having this value).
type ssoPending struct {
	nonce     string
	verifier  string
	expiresAt time.Time
}

// ssoManager owns the OIDC single sign-on mechanics: provider discovery, the
// OAuth2 authorization-code exchange, and ID-token verification. It holds no
// reference to the admin session store — the caller decides what a verified,
// authorized identity is allowed to do (mint a session, etc.), keeping this
// type a pure, testable OIDC client.
type ssoManager struct {
	oauthCfg     oauth2.Config
	verifier     *oidc.IDTokenVerifier
	allowedEmail string

	mu      sync.Mutex
	pending map[string]ssoPending
}

// newSSOManager performs OIDC discovery against cfg.OIDCIssuer and builds a
// ready-to-use manager, or returns an error if the issuer is unreachable or
// malformed. It is only called when all five MAILFOLD_OIDC_* variables are
// set (see openSSO in server.go); the caller treats a returned error the same
// as "feature unavailable" rather than failing the whole backend to start.
func newSSOManager(ctx context.Context, cfg *config.Config) (*ssoManager, error) {
	provider, err := oidc.NewProvider(ctx, cfg.OIDCIssuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery against %q: %w", cfg.OIDCIssuer, err)
	}
	return &ssoManager{
		oauthCfg: oauth2.Config{
			ClientID:     cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret,
			RedirectURL:  cfg.OIDCRedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "email"},
		},
		// ClientID set (rather than SkipClientIDCheck) so the verifier enforces
		// the token's audience matches this client, and every other default
		// check (signature, issuer, expiry) stays on.
		verifier:     provider.Verifier(&oidc.Config{ClientID: cfg.OIDCClientID}),
		allowedEmail: normalizeEmail(cfg.OIDCAllowedEmail),
		pending:      make(map[string]ssoPending),
	}, nil
}

// StartURL mints a fresh state/nonce/PKCE verifier, remembers them under the
// state key, and returns the URL to redirect the browser to.
func (m *ssoManager) StartURL() (string, error) {
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
	m.pending[state] = ssoPending{nonce: nonce, verifier: verifier, expiresAt: time.Now().Add(ssoPendingTTL)}
	m.mu.Unlock()

	return m.oauthCfg.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), nil
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

// Verify completes the callback leg: it redeems the pending state (rejecting
// unknown, expired, or already-used ones), exchanges the authorization code
// for tokens using the matching PKCE verifier, verifies the ID token's
// signature/issuer/audience/expiry (via the library's defaults) and its nonce
// (which the library deliberately leaves to the caller), and finally checks
// the verified email against the single configured allowed identity. It
// returns the verified email only when every one of those checks passes.
func (m *ssoManager) Verify(ctx context.Context, state, code string) (string, error) {
	pending, ok := m.consumePending(state)
	if !ok {
		return "", errors.New("sign-in expired or was already used; try again")
	}
	token, err := m.oauthCfg.Exchange(ctx, code, oauth2.VerifierOption(pending.verifier))
	if err != nil {
		return "", fmt.Errorf("code exchange failed: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return "", errors.New("the identity provider did not return an ID token")
	}
	idToken, err := m.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return "", fmt.Errorf("id token verification failed: %w", err)
	}
	if idToken.Nonce == "" || idToken.Nonce != pending.nonce {
		return "", errors.New("nonce mismatch")
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return "", fmt.Errorf("could not read identity claims: %w", err)
	}
	// Fail closed: a provider that omits email_verified entirely reads as
	// false here (Go's zero value), which is the safe direction for an admin
	// login gate — better to reject a working IdP misconfiguration loudly than
	// to accept an unverified email address as proof of identity.
	if !claims.EmailVerified || normalizeEmail(claims.Email) != m.allowedEmail {
		return "", errSSONotAuthorized
	}
	return claims.Email, nil
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
