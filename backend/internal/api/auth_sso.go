package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/domainadmin"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
)

// registerSSORoutes wires the OIDC single sign-on endpoints. All three are
// public (a caller cannot have a Mailfold token before completing this flow):
// providers lets the frontend feature-detect (and offer) SSO for whatever
// domain the user typed, start begins the redirect to the identity provider,
// and callback completes it, minting a WEBMAIL session for the mailbox that
// matches the verified identity — SSO here signs a mailbox user into their
// own webmail, not into the admin panel. Every route reports the feature as
// simply unavailable when s.sso is nil (no database configured, so there is
// nowhere to store provider configuration) — there is nothing sensitive to
// hide about that.
func (s *Server) registerSSORoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/sso/providers", s.handleSSOProvidersForDomain)
	mux.HandleFunc("GET /api/auth/sso/start", s.handleSSOStart)
	mux.HandleFunc("GET /api/auth/sso/callback", s.handleSSOCallback)
}

// handleSSOProvidersForDomain lets the frontend discover which SSO providers
// (if any) apply to the domain part of whatever the user typed into the
// login field, without exposing issuer/client configuration. An empty or
// missing domain, or SSO being unconfigured, both simply yield an empty list
// rather than an error — there is no sign-in option to offer, that's all.
func (s *Server) handleSSOProvidersForDomain(w http.ResponseWriter, r *http.Request) {
	type providerOption struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	domain := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("domain")))
	if s.sso == nil || domain == "" {
		writeJSON(w, http.StatusOK, []providerOption{})
		return
	}
	providers, err := s.domainAdminStore.ProvidersForDomain(domain)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]providerOption, 0, len(providers))
	for _, p := range providers {
		out = append(out, providerOption{ID: p.ID, Name: p.Name})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleSSOStart redirects the browser to the identity provider named by
// provider_id to begin the authorization-code flow.
func (s *Server) handleSSOStart(w http.ResponseWriter, r *http.Request) {
	if s.sso == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "SSO is not configured"})
		return
	}
	id, err := strconv.ParseInt(r.URL.Query().Get("provider_id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid provider_id"})
		return
	}
	dest, err := s.sso.StartURL(r.Context(), id)
	if err != nil {
		s.logger.Warn("sso start failed", "error", err, "provider_id", id)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "could not start sign-in"})
		return
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

// ssoGenericFailureMsg is shown for every security-relevant callback failure
// (provider error, expired/reused state, bad token, mailbox not allowed) so
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

// resolveSSOMailbox turns a verified identity into the one mailbox it is
// allowed to sign into: the verified email must match an active mailbox
// exactly, and that mailbox's domain must be allowed by the provider that
// vouched for the identity (every domain, if the provider is shared; only its
// configured domains, if scoped) — a domain-scoped provider authenticating
// into a mailbox outside its scope is rejected even though the email matched,
// which is the whole point of domain-scoping a provider in the first place.
func (s *Server) resolveSSOMailbox(ctx context.Context, identity VerifiedIdentity) (*mailcow.Mailbox, error) {
	p, ok, err := s.domainAdminStore.GetProvider(identity.ProviderID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errSSOMailboxNotAllowed
	}
	at := strings.LastIndexByte(identity.Email, '@')
	if at < 0 || !providerCoversDomain(p, identity.Email[at+1:]) {
		return nil, errSSOMailboxNotAllowed
	}
	mailboxes, err := s.mc.Mailboxes(ctx)
	if err != nil {
		return nil, err
	}
	if mbox := activeMailboxByEmail(mailboxes, identity.Email); mbox != nil {
		return mbox, nil
	}
	return nil, errSSOMailboxNotAllowed
}

// providerCoversDomain reports whether p is allowed to vouch for a mailbox on
// domain — true for every domain when p is shared, otherwise only for its
// explicitly configured domains.
func providerCoversDomain(p domainadmin.Provider, domain string) bool {
	if p.AllDomains {
		return true
	}
	for _, d := range p.Domains {
		if strings.EqualFold(d, domain) {
			return true
		}
	}
	return false
}

// activeMailboxByEmail returns the active mailbox whose username matches
// email (case-insensitively), or nil if there is none.
func activeMailboxByEmail(mailboxes []mailcow.Mailbox, email string) *mailcow.Mailbox {
	for i := range mailboxes {
		if mailboxes[i].Active == 1 && strings.EqualFold(mailboxes[i].Username, email) {
			return &mailboxes[i]
		}
	}
	return nil
}

// ssoWebmailCredential returns a working IMAP/SMTP credential for mailbox —
// SSO never has (or needs) the mailbox's real password, so it mints a mailcow
// app-password the first time a mailbox signs in this way and reuses the
// cached one thereafter (see internal/domainadmin's sso_mailbox_credential
// table), the same pattern internal/apikey uses to front a mailbox without
// storing its password.
func (s *Server) ssoWebmailCredential(ctx context.Context, mailbox string) (string, error) {
	if cred, ok, err := s.domainAdminStore.GetMailboxCredential(mailbox); err != nil {
		return "", err
	} else if ok {
		if pw, err := s.adminCipher.Open(cred.AppPasswdEnc, cred.AppPasswdNonce); err == nil {
			return string(pw), nil
		}
		// Decryption failed (e.g. the master key rotated) — fall through and
		// mint a fresh app-password rather than failing the login outright.
	}

	appPw, err := randomAppPassword()
	if err != nil {
		return "", err
	}
	appName := "mailfold-sso:" + mailbox
	results, err := s.mc.AddAppPassword(ctx, map[string]any{
		"username":    mailbox,
		"app_name":    appName,
		"app_passwd":  appPw,
		"app_passwd2": appPw,
		"active":      "1",
		"protocols":   appPwProtocols,
	})
	if err != nil {
		return "", err
	}
	if ok, _ := mailcow.ResultsOK(results); !ok {
		return "", errors.New("mailcow rejected app-password creation")
	}
	appPwID, err := s.recoverAppPwID(ctx, mailbox, appName)
	if err != nil {
		s.deleteAppPwByName(ctx, mailbox, appName)
		return "", err
	}
	enc, nonce, err := s.adminCipher.Seal([]byte(appPw))
	if err != nil {
		s.deleteAppPwByName(ctx, mailbox, appName)
		return "", err
	}
	if err := s.domainAdminStore.SetMailboxCredential(mailbox, appPwID, enc, nonce, time.Now()); err != nil {
		return "", err
	}
	return appPw, nil
}

// handleSSOCallback completes the authorization-code exchange and, on a fully
// verified identity that resolves to an allowed mailbox, mints a webmail
// session for it exactly the way a password login does. Every failure — a
// provider-reported error, an expired/reused state, a bad token, or a mailbox
// that isn't allowed — redirects with the same generic message; the specific
// reason is logged server-side only, so the browser (and anyone watching over
// the user's shoulder) never learns which check failed.
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

	ctx := r.Context()
	identity, err := s.sso.Verify(ctx, q.Get("state"), q.Get("code"))
	if err != nil {
		s.logger.Warn("sso callback failed", "error", err)
		ssoErrorRedirect(w, r, ssoGenericFailureMsg)
		return
	}
	mbox, err := s.resolveSSOMailbox(ctx, identity)
	if err != nil {
		s.logger.Warn("sso callback: identity did not resolve to an allowed mailbox", "error", err, "email", identity.Email)
		ssoErrorRedirect(w, r, ssoGenericFailureMsg)
		return
	}
	appPw, err := s.ssoWebmailCredential(ctx, mbox.Username)
	if err != nil {
		s.logger.Error("sso: failed to obtain a webmail credential", "error", err, "mailbox", mbox.Username)
		ssoErrorRedirect(w, r, ssoGenericFailureMsg)
		return
	}
	token, _, err := s.webmailSessions.Create(mbox.Username, appPw)
	if err != nil {
		s.logger.Error("sso: failed to mint webmail session", "error", err)
		ssoErrorRedirect(w, r, ssoGenericFailureMsg)
		return
	}
	ssoFrontendRedirect(w, r, "sso_webmail_token="+url.QueryEscape(token)+"&sso_webmail_email="+url.QueryEscape(mbox.Username))
}
