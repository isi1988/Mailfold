package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/descope/virtualwebauthn"

	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
)

// newWebAuthnTestServer builds a server with a real (non-wildcard) CORS origin,
// since openWebAuthn refuses to enable passkeys against the wildcard "*"
// newAccountTestServer otherwise uses — WebAuthn needs a single, fixed
// relying-party ID to verify against.
func newWebAuthnTestServer(t *testing.T) (http.Handler, *Server) {
	t.Helper()
	cfg := &config.Config{
		MailcowBaseURL:  mockMailcow(t, 0, "").URL,
		MailcowAPIKey:   "k",
		AdminUser:       "admin",
		AdminPassword:   "pw",
		SessionTTL:      time.Hour,
		CORSOrigins:     []string{"https://mailfold.example"},
		LoginRateMax:    1000,
		LoginRateWindow: time.Minute,
		MaxBodyBytes:    1 << 20,
		DBDriver:        "sqlite",
		DBPath:          t.TempDir() + "/admin.db",
	}
	mc := mailcow.NewClient(cfg.MailcowBaseURL, cfg.MailcowAPIKey, false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	limiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(cfg, mc, authn, limiter, logger)
	return srv.Handler(), srv
}

func TestWebAuthnDisabledWithWildcardCORS(t *testing.T) {
	// newAccountTestServer's default CORSOrigins is ["*"], which can't produce
	// a stable relying-party ID, so passkeys must stay off.
	h, srv, _ := newAccountTestServer(t, accountTestOpts{withDB: true})
	if srv.webAuthn != nil {
		t.Fatal("expected WebAuthn to be disabled with a wildcard CORS origin")
	}
	token := loginToken(t, h)
	rec := do(h, http.MethodPost, "/api/account/webauthn/register/begin", token, "")
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("want 501 when WebAuthn is disabled, got %d", rec.Code)
	}
}

// TestWebAuthnEnrollAndLogin exercises the whole passkey lifecycle end to end
// using a virtual (software) authenticator: enroll a credential while signed
// in, confirm it now gates login behind a second factor exactly like TOTP,
// and complete a login using only the passkey assertion.
func TestWebAuthnEnrollAndLogin(t *testing.T) {
	h, srv := newWebAuthnTestServer(t)
	if srv.webAuthn == nil {
		t.Fatal("expected WebAuthn to be enabled with a non-wildcard CORS origin")
	}

	rp := virtualwebauthn.RelyingParty{Name: "Mailfold", ID: "mailfold.example", Origin: "https://mailfold.example"}
	authenticator := virtualwebauthn.NewAuthenticator()
	cred := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)

	// Enroll while signed in normally (no 2FA yet, so loginToken works as-is).
	token := loginToken(t, h)

	rec := do(h, http.MethodPost, "/api/account/webauthn/register/begin", token, `{"current_password":"wrong"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("register/begin with wrong password: want 403, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = do(h, http.MethodPost, "/api/account/webauthn/register/begin", token, `{"current_password":"pw"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("register/begin: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	attestationOptions, err := virtualwebauthn.ParseAttestationOptions(rec.Body.String())
	if err != nil {
		t.Fatalf("ParseAttestationOptions: %v", err)
	}
	authenticator.AddCredential(cred)
	attestationResponse := virtualwebauthn.CreateAttestationResponse(rp, authenticator, cred, *attestationOptions)

	rec = do(h, http.MethodPost, "/api/account/webauthn/register/finish?name=Test+Key", token, attestationResponse)
	if rec.Code != http.StatusOK {
		t.Fatalf("register/finish: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// The new credential shows up in the list, without exposing its raw id or
	// public key.
	rec = do(h, http.MethodGet, "/api/account/webauthn/credentials", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d", rec.Code)
	}
	var creds []credentialSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &creds); err != nil {
		t.Fatalf("decode credentials: %v", err)
	}
	if len(creds) != 1 || creds[0].Name != "Test Key" {
		t.Fatalf("want 1 credential named %q, got %+v", "Test Key", creds)
	}

	// Password-only login must now require a second factor, exactly like TOTP.
	rec = do(h, http.MethodPost, "/api/auth/login", "", `{"user":"admin","password":"pw"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var loginOut struct {
		Needs2FA     bool     `json:"needs_2fa"`
		PendingToken string   `json:"pending_token"`
		Methods      []string `json:"methods"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &loginOut); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if !loginOut.Needs2FA || loginOut.PendingToken == "" {
		t.Fatalf("want needs_2fa with a pending token now that a passkey is enrolled, got %+v", loginOut)
	}
	if len(loginOut.Methods) != 1 || loginOut.Methods[0] != "webauthn" {
		t.Fatalf("want methods=[webauthn] with only a passkey enrolled, got %+v", loginOut.Methods)
	}

	// Complete the login with the passkey assertion.
	rec = do(h, http.MethodPost, "/api/auth/2fa/webauthn/begin", "", `{"pending_token":"`+loginOut.PendingToken+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("2fa/webauthn/begin: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	assertionOptions, err := virtualwebauthn.ParseAssertionOptions(rec.Body.String())
	if err != nil {
		t.Fatalf("ParseAssertionOptions: %v", err)
	}
	assertionResponse := virtualwebauthn.CreateAssertionResponse(rp, authenticator, cred, *assertionOptions)

	rec = do(h, http.MethodPost, "/api/auth/2fa/webauthn/finish?pending_token="+loginOut.PendingToken, "", assertionResponse)
	if rec.Code != http.StatusOK {
		t.Fatalf("2fa/webauthn/finish: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var sessOut struct {
		Token string `json:"token"`
		User  string `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &sessOut); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if sessOut.Token == "" || sessOut.User != "admin" {
		t.Fatalf("want a minted session for admin, got %+v", sessOut)
	}

	// The redeemed pending token must not be reusable a second time.
	rec = do(h, http.MethodPost, "/api/auth/2fa/webauthn/finish?pending_token="+loginOut.PendingToken, "", assertionResponse)
	if rec.Code == http.StatusOK {
		t.Fatal("a redeemed pending token should not verify again")
	}

	// Deleting the credential removes the second-factor requirement again.
	rec = do(h, http.MethodDelete, "/api/account/webauthn/credentials/"+strconv.FormatInt(creds[0].ID, 10), sessOut.Token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = do(h, http.MethodPost, "/api/auth/login", "", `{"user":"admin","password":"pw"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login after delete: want 200, got %d", rec.Code)
	}
	var finalOut struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &finalOut)
	if finalOut.Token == "" {
		t.Fatal("login should mint a session directly once no passkey is enrolled")
	}
}

// TestWebAuthnLoginWithBackupEligibleCredential covers a synced passkey (an
// iCloud Keychain or Google Password Manager credential, simulated here via
// BackupEligible: true) — go-webauthn hard-rejects a login whose assertion
// reports a different BackupEligible value than what was stored at
// registration, so this fails unless that flag is round-tripped through
// storage between the register and login ceremonies (see
// WebAuthnCredential.BackupEligible's doc comment). A real Touch ID passkey
// with iCloud Keychain sync enabled hit exactly this failure in production.
func TestWebAuthnLoginWithBackupEligibleCredential(t *testing.T) {
	h, srv := newWebAuthnTestServer(t)
	if srv.webAuthn == nil {
		t.Fatal("expected WebAuthn to be enabled with a non-wildcard CORS origin")
	}

	rp := virtualwebauthn.RelyingParty{Name: "Mailfold", ID: "mailfold.example", Origin: "https://mailfold.example"}
	authenticator := virtualwebauthn.NewAuthenticatorWithOptions(virtualwebauthn.AuthenticatorOptions{BackupEligible: true})
	cred := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)

	token := loginToken(t, h)
	rec := do(h, http.MethodPost, "/api/account/webauthn/register/begin", token, `{"current_password":"pw"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("register/begin: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	attestationOptions, err := virtualwebauthn.ParseAttestationOptions(rec.Body.String())
	if err != nil {
		t.Fatalf("ParseAttestationOptions: %v", err)
	}
	authenticator.AddCredential(cred)
	attestationResponse := virtualwebauthn.CreateAttestationResponse(rp, authenticator, cred, *attestationOptions)

	rec = do(h, http.MethodPost, "/api/account/webauthn/register/finish?name=Synced+Key", token, attestationResponse)
	if rec.Code != http.StatusOK {
		t.Fatalf("register/finish: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = do(h, http.MethodPost, "/api/auth/login", "", `{"user":"admin","password":"pw"}`)
	var loginOut struct {
		PendingToken string `json:"pending_token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &loginOut)

	rec = do(h, http.MethodPost, "/api/auth/2fa/webauthn/begin", "", `{"pending_token":"`+loginOut.PendingToken+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("2fa/webauthn/begin: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	assertionOptions, err := virtualwebauthn.ParseAssertionOptions(rec.Body.String())
	if err != nil {
		t.Fatalf("ParseAssertionOptions: %v", err)
	}
	assertionResponse := virtualwebauthn.CreateAssertionResponse(rp, authenticator, cred, *assertionOptions)

	rec = do(h, http.MethodPost, "/api/auth/2fa/webauthn/finish?pending_token="+loginOut.PendingToken, "", assertionResponse)
	if rec.Code != http.StatusOK {
		t.Fatalf("a backup-eligible passkey must still verify on login: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var sessOut struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &sessOut); err != nil || sessOut.Token == "" {
		t.Fatalf("decode session: err=%v body=%s", err, rec.Body.String())
	}
}
