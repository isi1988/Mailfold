package api

import (
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // matches the RFC 6238 TOTP algorithm being tested against
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime/quotedprintable"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-imap/backend/memory"
	"github.com/emersion/go-imap/server"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
)

// accountTestOpts controls which optional subsystems newAccountTestServer wires
// up, so each test only pays for the fakes it actually needs.
type accountTestOpts struct {
	withDB     bool
	withEncKey bool
	withIMAP   bool // also starts SMTP; both are needed for a working notify sender
}

// capturingSMTP is a minimal SMTP server that accepts any credentials and
// records every message body, so a test can pull a password-reset link back
// out of the "email" that was sent.
type capturingSMTP struct {
	mu       sync.Mutex
	count    int
	lastBody string
}

func (b *capturingSMTP) NewSession(_ *smtp.Conn) (smtp.Session, error) {
	return &capturingSMTPSession{b: b}, nil
}

type capturingSMTPSession struct{ b *capturingSMTP }

func (s *capturingSMTPSession) AuthMechanisms() []string { return []string{sasl.Plain} }
func (s *capturingSMTPSession) Auth(string) (sasl.Server, error) {
	return sasl.NewPlainServer(func(_, _, _ string) error { return nil }), nil
}
func (s *capturingSMTPSession) Mail(string, *smtp.MailOptions) error { return nil }
func (s *capturingSMTPSession) Rcpt(string, *smtp.RcptOptions) error { return nil }
func (s *capturingSMTPSession) Data(r io.Reader) error {
	body, _ := io.ReadAll(r)
	s.b.mu.Lock()
	s.b.count++
	s.b.lastBody = string(body)
	s.b.mu.Unlock()
	return nil
}
func (s *capturingSMTPSession) Reset()        {}
func (s *capturingSMTPSession) Logout() error { return nil }

func (b *capturingSMTP) sent() (int, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.count, b.lastBody
}

func newAccountTestServer(t *testing.T, opts accountTestOpts) (http.Handler, *Server, *capturingSMTP) {
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
		MaxBodyBytes:    1 << 20,
	}
	if opts.withDB {
		cfg.DBDriver = "sqlite"
		cfg.DBPath = t.TempDir() + "/admin.db"
	}
	if opts.withEncKey {
		key := make([]byte, 32)
		for i := range key {
			key[i] = byte(i + 7)
		}
		cfg.AdminEncKey = key
	}
	var smtpBE *capturingSMTP
	if opts.withIMAP {
		imapSrv := server.New(memory.New())
		imapSrv.AllowInsecureAuth = true
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		go func() { _ = imapSrv.Serve(ln) }()
		t.Cleanup(func() { _ = imapSrv.Close() })
		cfg.IMAPAddr = ln.Addr().String()
		cfg.MailUseTLS = false

		smtpBE = &capturingSMTP{}
		srv := smtp.NewServer(smtpBE)
		srv.AllowInsecureAuth = true
		sln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		go func() { _ = srv.Serve(sln) }()
		t.Cleanup(func() { _ = srv.Close() })
		cfg.SMTPAddr = sln.Addr().String()
	}

	mc := mailcow.NewClient(cfg.MailcowBaseURL, cfg.MailcowAPIKey, false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	limiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(cfg, mc, authn, limiter, logger)
	return srv.Handler(), srv, smtpBE
}

// seedNotifySender writes the notify-sender fields directly through the store
// and cipher, bypassing the PUT endpoint's live IMAP verification. It exists
// because the fake IMAP backend (see newAccountTestServer) only recognises a
// single hardcoded login ("username", with no "@"), which a real, spec-compliant
// SMTP server correctly refuses as a MAIL FROM address — a constraint of the
// test double, not of Mailfold. IMAP verification itself is exercised directly
// against that fake backend in TestNotifySenderAndPasswordResetFlow; this helper
// lets the send-path tests use a realistic, address-shaped identity instead.
func seedNotifySender(t *testing.T, srv *Server, mailbox, password string) {
	t.Helper()
	enc, nonce, err := srv.adminCipher.Seal([]byte(password))
	if err != nil {
		t.Fatalf("seedNotifySender: Seal: %v", err)
	}
	if err := srv.adminStore.SetNotifySender(srv.cfg.AdminUser, mailbox, enc, nonce, time.Now()); err != nil {
		t.Fatalf("seedNotifySender: SetNotifySender: %v", err)
	}
}

func TestAccountEndpointsWithoutDB(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{})
	token := loginToken(t, h)

	for _, path := range []string{"/api/account/profile"} {
		if rec := do(h, http.MethodGet, path, token, ""); rec.Code != http.StatusNotImplemented {
			t.Errorf("GET %s without DB = %d, want 501", path, rec.Code)
		}
	}
	if rec := do(h, http.MethodPost, "/api/account/password", token, `{"current_password":"pw","new_password":"newlongpw"}`); rec.Code != http.StatusNotImplemented {
		t.Errorf("password change without DB = %d, want 501", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/account/2fa/enroll", token, `{"current_password":"pw"}`); rec.Code != http.StatusNotImplemented {
		t.Errorf("2fa enroll without DB = %d, want 501", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/account/notify-sender", token, ""); rec.Code != http.StatusNotImplemented {
		t.Errorf("notify-sender without DB = %d, want 501", rec.Code)
	}

	// Sessions are backed by the in-memory authenticator, not the admin store,
	// so they must keep working even with no database configured.
	if rec := do(h, http.MethodGet, "/api/account/sessions", token, ""); rec.Code != http.StatusOK {
		t.Errorf("sessions without DB = %d, want 200", rec.Code)
	}
}

func TestAccountProfileAndPasswordChange(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{withDB: true})
	token := loginToken(t, h)

	rec := do(h, http.MethodPut, "/api/account/profile", token,
		`{"display_name":"Admin","email":"admin@example.com","timezone":"UTC","avatar_url":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile put = %d", rec.Code)
	}
	rec = do(h, http.MethodGet, "/api/account/profile", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("profile get = %d", rec.Code)
	}
	var prof profileResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &prof)
	if prof.DisplayName != "Admin" || prof.Email != "admin@example.com" {
		t.Errorf("unexpected profile: %+v", prof)
	}

	// Wrong current password is rejected.
	if rec := do(h, http.MethodPost, "/api/account/password", token, `{"current_password":"wrong","new_password":"newlongpw"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("password change with wrong current pw = %d, want 401", rec.Code)
	}
	// Too-short new password is rejected.
	if rec := do(h, http.MethodPost, "/api/account/password", token, `{"current_password":"pw","new_password":"short"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("password change with short new pw = %d, want 400", rec.Code)
	}
	// Successful change takes effect immediately.
	if rec := do(h, http.MethodPost, "/api/account/password", token, `{"current_password":"pw","new_password":"newlongpw"}`); rec.Code != http.StatusOK {
		t.Fatalf("password change = %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/auth/login", "", `{"user":"admin","password":"pw"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("old password should no longer work, got %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/auth/login", "", `{"user":"admin","password":"newlongpw"}`); rec.Code != http.StatusOK {
		t.Errorf("new password should work, got %d", rec.Code)
	}
}

func TestSessionsListAndRevoke(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{})
	tok1 := loginToken(t, h)
	tok2 := loginToken(t, h)

	rec := do(h, http.MethodGet, "/api/account/sessions", tok1, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("sessions list = %d", rec.Code)
	}
	var sessions []auth.SessionInfo
	_ = json.Unmarshal(rec.Body.Bytes(), &sessions)
	if len(sessions) != 2 {
		t.Fatalf("want 2 sessions, got %d", len(sessions))
	}
	var otherID, currentID string
	for _, si := range sessions {
		if si.Current {
			currentID = si.ID
		} else {
			otherID = si.ID
		}
	}
	if currentID == "" || otherID == "" {
		t.Fatalf("expected exactly one current session: %+v", sessions)
	}

	// Revoking the current session from this endpoint is rejected.
	if rec := do(h, http.MethodPost, "/api/account/sessions/"+currentID+"/revoke", tok1, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("revoking current session = %d, want 400", rec.Code)
	}
	// Revoking an unknown id 404s.
	if rec := do(h, http.MethodPost, "/api/account/sessions/does-not-exist/revoke", tok1, ""); rec.Code != http.StatusNotFound {
		t.Errorf("revoking unknown id = %d, want 404", rec.Code)
	}
	// Revoking the other session works, and that token stops working.
	if rec := do(h, http.MethodPost, "/api/account/sessions/"+otherID+"/revoke", tok1, ""); rec.Code != http.StatusOK {
		t.Errorf("revoke other session = %d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/auth/me", tok2, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked session should be unauthorized, got %d", rec.Code)
	}

	// revoke-all-except leaves only the calling session valid.
	tok3 := loginToken(t, h)
	rec = do(h, http.MethodPost, "/api/account/sessions/revoke-all", tok1, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke-all = %d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/auth/me", tok3, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("tok3 should be revoked, got %d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/auth/me", tok1, ""); rec.Code != http.StatusOK {
		t.Errorf("calling session should survive revoke-all, got %d", rec.Code)
	}
}

func TestTOTPRequiresEncKey(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{withDB: true}) // no AdminEncKey
	token := loginToken(t, h)
	if rec := do(h, http.MethodPost, "/api/account/2fa/enroll", token, `{"current_password":"pw"}`); rec.Code != http.StatusNotImplemented {
		t.Errorf("enroll without enc key = %d, want 501", rec.Code)
	}
}

func TestTOTPEnrollConfirmLoginDisable(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{withDB: true, withEncKey: true})
	token := loginToken(t, h)

	// Wrong current password blocks enrollment.
	if rec := do(h, http.MethodPost, "/api/account/2fa/enroll", token, `{"current_password":"wrong"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("enroll wrong pw = %d, want 401", rec.Code)
	}

	rec := do(h, http.MethodPost, "/api/account/2fa/enroll", token, `{"current_password":"pw"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("enroll = %d", rec.Code)
	}
	var enroll struct {
		Secret    string `json:"secret"`
		QRDataURI string `json:"qr_data_uri"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &enroll)
	if enroll.Secret == "" || !strings.HasPrefix(enroll.QRDataURI, "data:image/png;base64,") {
		t.Fatalf("unexpected enroll response: %+v", enroll)
	}

	// A wrong code does not confirm enrollment.
	if rec := do(h, http.MethodPost, "/api/account/2fa/confirm", token, `{"code":"000000"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("confirm wrong code = %d, want 401", rec.Code)
	}

	rec = do(h, http.MethodPost, "/api/account/2fa/confirm", token, fmt.Sprintf(`{"code":%q}`, totpCodeFor(t, enroll.Secret)))
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm = %d body=%s", rec.Code, rec.Body.String())
	}
	var confirmed struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &confirmed)
	if len(confirmed.RecoveryCodes) != 10 {
		t.Fatalf("want 10 recovery codes, got %d", len(confirmed.RecoveryCodes))
	}

	// A fresh login now requires the second factor.
	rec = do(h, http.MethodPost, "/api/auth/login", "", `{"user":"admin","password":"pw"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login = %d", rec.Code)
	}
	var pending struct {
		Needs2FA     bool   `json:"needs_2fa"`
		PendingToken string `json:"pending_token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &pending)
	if !pending.Needs2FA || pending.PendingToken == "" {
		t.Fatalf("expected a pending 2FA challenge, got %+v", pending)
	}

	// A wrong code at the verify step is rejected — and, since a pending token
	// is single-use regardless of outcome, it also burns this pending login, so
	// every following attempt below starts from a fresh /api/auth/login call.
	badBody := fmt.Sprintf(`{"pending_token":%q,"code":"000000"}`, pending.PendingToken)
	if rec := do(h, http.MethodPost, "/api/auth/2fa/verify", "", badBody); rec.Code != http.StatusUnauthorized {
		t.Errorf("2fa verify wrong code = %d, want 401", rec.Code)
	}

	rec = do(h, http.MethodPost, "/api/auth/login", "", `{"user":"admin","password":"pw"}`)
	_ = json.Unmarshal(rec.Body.Bytes(), &pending)
	goodBody := fmt.Sprintf(`{"pending_token":%q,"code":%q}`, pending.PendingToken, totpCodeFor(t, enroll.Secret))
	rec = do(h, http.MethodPost, "/api/auth/2fa/verify", "", goodBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("2fa verify = %d", rec.Code)
	}
	var sess struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sess)
	if sess.Token == "" {
		t.Fatal("expected a session token after 2fa verify")
	}

	// The pending token is single-use.
	if rec := do(h, http.MethodPost, "/api/auth/2fa/verify", "", goodBody); rec.Code != http.StatusUnauthorized {
		t.Errorf("reusing a pending token = %d, want 401", rec.Code)
	}

	// A recovery code also redeems a fresh pending login.
	rec = do(h, http.MethodPost, "/api/auth/login", "", `{"user":"admin","password":"pw"}`)
	_ = json.Unmarshal(rec.Body.Bytes(), &pending)
	recoveryBody := fmt.Sprintf(`{"pending_token":%q,"code":%q}`, pending.PendingToken, confirmed.RecoveryCodes[0])
	if rec := do(h, http.MethodPost, "/api/auth/2fa/verify", "", recoveryBody); rec.Code != http.StatusOK {
		t.Fatalf("recovery code verify = %d, body=%s", rec.Code, rec.Body.String())
	}

	// The same recovery code is rejected on a second, otherwise-valid pending
	// login (a fresh pending token, so this isolates the recovery code itself
	// being single-use rather than the pending token).
	rec = do(h, http.MethodPost, "/api/auth/login", "", `{"user":"admin","password":"pw"}`)
	_ = json.Unmarshal(rec.Body.Bytes(), &pending)
	reuseBody := fmt.Sprintf(`{"pending_token":%q,"code":%q}`, pending.PendingToken, confirmed.RecoveryCodes[0])
	if rec := do(h, http.MethodPost, "/api/auth/2fa/verify", "", reuseBody); rec.Code != http.StatusUnauthorized {
		t.Errorf("reusing a spent recovery code = %d, want 401", rec.Code)
	}

	// Recovery codes can be regenerated (requires 2FA already enabled).
	rec = do(h, http.MethodPost, "/api/account/2fa/recovery-codes", sess.Token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("recovery regenerate = %d", rec.Code)
	}
	var regenerated struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &regenerated)
	if len(regenerated.RecoveryCodes) != 10 {
		t.Fatalf("want 10 regenerated codes, got %d", len(regenerated.RecoveryCodes))
	}

	// Disabling requires the current password and turns 2FA fully off.
	if rec := do(h, http.MethodPost, "/api/account/2fa/disable", sess.Token, `{"current_password":"wrong"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("disable wrong pw = %d, want 401", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/account/2fa/disable", sess.Token, `{"current_password":"pw"}`); rec.Code != http.StatusOK {
		t.Fatalf("disable = %d", rec.Code)
	}
	rec = do(h, http.MethodPost, "/api/auth/login", "", `{"user":"admin","password":"pw"}`)
	var direct struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &direct)
	if rec.Code != http.StatusOK || direct.Token == "" {
		t.Errorf("login should no longer require 2FA after disable, code=%d body=%s", rec.Code, rec.Body.String())
	}

	// Regenerating recovery codes once 2FA is off is rejected.
	if rec := do(h, http.MethodPost, "/api/account/2fa/recovery-codes", direct.Token, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("recovery regenerate after disable = %d, want 400", rec.Code)
	}
}

func TestNotifySenderRequiresIMAP(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{withDB: true, withEncKey: true}) // no IMAP/SMTP
	token := loginToken(t, h)
	body := `{"current_password":"pw","mailbox":"username","password":"password"}`
	if rec := do(h, http.MethodPut, "/api/account/notify-sender", token, body); rec.Code != http.StatusNotImplemented {
		t.Errorf("notify-sender put without mail config = %d, want 501", rec.Code)
	}
}

func TestNotifySenderAndPasswordResetFlow(t *testing.T) {
	h, srv, smtpBE := newAccountTestServer(t, accountTestOpts{withDB: true, withEncKey: true, withIMAP: true})
	token := loginToken(t, h)

	// Wrong current admin password is rejected.
	body := `{"current_password":"wrong","mailbox":"username","password":"password"}`
	if rec := do(h, http.MethodPut, "/api/account/notify-sender", token, body); rec.Code != http.StatusUnauthorized {
		t.Errorf("notify-sender put wrong admin pw = %d, want 401", rec.Code)
	}
	// A mailbox that fails IMAP verification is rejected.
	badMailbox := `{"current_password":"pw","mailbox":"username","password":"not-the-real-password"}`
	if rec := do(h, http.MethodPut, "/api/account/notify-sender", token, badMailbox); rec.Code != http.StatusUnauthorized {
		t.Errorf("notify-sender put unverifiable mailbox = %d, want 401", rec.Code)
	}
	// The real mailbox credentials succeed.
	goodMailbox := `{"current_password":"pw","mailbox":"username","password":"password"}`
	if rec := do(h, http.MethodPut, "/api/account/notify-sender", token, goodMailbox); rec.Code != http.StatusOK {
		t.Fatalf("notify-sender put = %d", rec.Code)
	}
	rec := do(h, http.MethodGet, "/api/account/notify-sender", token, "")
	var ns notifySenderResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &ns)
	if !ns.Configured || ns.Mailbox != "username" {
		t.Fatalf("unexpected notify sender state: %+v", ns)
	}

	// A test send needs a profile email first.
	if rec := do(h, http.MethodPost, "/api/account/notify-sender/test", token, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("test send without profile email = %d, want 400", rec.Code)
	}
	if rec := do(h, http.MethodPut, "/api/account/profile", token, `{"display_name":"","email":"admin@example.com","timezone":"","avatar_url":""}`); rec.Code != http.StatusOK {
		t.Fatalf("profile put = %d", rec.Code)
	}

	// The fake IMAP backend's one hardcoded login ("username") has no "@" in it,
	// which a real SMTP server correctly refuses as a MAIL FROM address (RFC
	// 5321 requires a Mailbox with a domain) — that constraint belongs to the
	// test double, not to Mailfold, so the actual send-path checks below swap in
	// an address-shaped sender via seedNotifySender. IMAP verification itself
	// was already fully exercised above (wrong/right mailbox credentials).
	seedNotifySender(t, srv, "sender@example.com", "secret")
	if rec := do(h, http.MethodPost, "/api/account/notify-sender/test", token, ""); rec.Code != http.StatusOK {
		t.Fatalf("test send = %d body=%s", rec.Code, rec.Body.String())
	}
	if got, _ := smtpBE.sent(); got != 1 {
		t.Fatalf("want 1 message sent by the test endpoint, got %d", got)
	}

	// Forgot-password always answers 200, and actually emails a reset link now
	// that a notify sender and a profile email are both configured.
	rec = do(h, http.MethodPost, "/api/auth/forgot-password", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("forgot-password = %d", rec.Code)
	}
	count, lastBody := smtpBE.sent()
	if count != 2 {
		t.Fatalf("want 2 messages sent total (test + reset), got %d", count)
	}
	tok := extractResetToken(t, lastBody)

	// Reject too-short new passwords before even touching the token.
	if rec := do(h, http.MethodPost, "/api/auth/reset-password", "", fmt.Sprintf(`{"token":%q,"new_password":"short"}`, tok)); rec.Code != http.StatusBadRequest {
		t.Errorf("reset with short password = %d, want 400", rec.Code)
	}
	// An unknown token is rejected.
	if rec := do(h, http.MethodPost, "/api/auth/reset-password", "", `{"token":"deadbeef","new_password":"resetpassword1"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("reset with unknown token = %d, want 400", rec.Code)
	}
	// The real token resets the password.
	if rec := do(h, http.MethodPost, "/api/auth/reset-password", "", fmt.Sprintf(`{"token":%q,"new_password":"resetpassword1"}`, tok)); rec.Code != http.StatusOK {
		t.Fatalf("reset-password = %d, body=%s", rec.Code, rec.Body.String())
	}
	// The token is single-use.
	if rec := do(h, http.MethodPost, "/api/auth/reset-password", "", fmt.Sprintf(`{"token":%q,"new_password":"anotherpassword1"}`, tok)); rec.Code != http.StatusBadRequest {
		t.Errorf("reusing the reset token = %d, want 400", rec.Code)
	}
	// The old admin session was revoked as part of the reset.
	if rec := do(h, http.MethodGet, "/api/auth/me", token, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("pre-reset session should be revoked, got %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/auth/login", "", `{"user":"admin","password":"resetpassword1"}`); rec.Code != http.StatusOK {
		t.Errorf("login with the reset password = %d, want 200", rec.Code)
	}
}

func TestForgotPasswordSilentWithoutNotifySender(t *testing.T) {
	h, _, smtpBE := newAccountTestServer(t, accountTestOpts{withDB: true, withEncKey: true, withIMAP: true})
	// No notify sender configured yet.
	rec := do(h, http.MethodPost, "/api/auth/forgot-password", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("forgot-password without sender = %d, want 200 (always)", rec.Code)
	}
	if got, _ := smtpBE.sent(); got != 0 {
		t.Errorf("no email should have been sent, got %d", got)
	}
}

var resetTokenPattern = regexp.MustCompile(`token=([0-9a-f]+)`)

// extractResetToken pulls the reset token back out of the raw MIME message
// captured by capturingSMTP. The text/plain body go-message renders is
// quoted-printable encoded by default (its automatic choice for a body
// containing "="), which would otherwise turn "token=" into "token=3D" and can
// hard-wrap the hex token across a soft line break, so the raw bytes are
// decoded first.
func extractResetToken(t *testing.T, rawBody string) string {
	t.Helper()
	decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(rawBody)))
	if err != nil {
		t.Fatalf("decode quoted-printable body: %v", err)
	}
	m := resetTokenPattern.FindStringSubmatch(string(decoded))
	if len(m) != 2 {
		t.Fatalf("could not find a reset token in the sent email:\nraw:\n%s\ndecoded:\n%s", rawBody, decoded)
	}
	return m[1]
}

// totpCodeFor computes the current RFC 6238 code for a base32 secret,
// independently of the admin package's own (unexported) implementation, so the
// HTTP-level test exercises the real wire format rather than reusing internals.
func totpCodeFor(t *testing.T, secret string) string {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	counter := uint64(time.Now().Unix() / 30)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	trunc := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	code := trunc % uint32(math.Pow10(6))
	return fmt.Sprintf("%06d", code)
}
