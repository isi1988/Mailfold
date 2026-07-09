package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/backend/memory"
	imapserver "github.com/emersion/go-imap/server"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"

	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
	"github.com/isi1988/Mailfold/backend/internal/webmail"
)

// acceptingSMTP is a minimal SMTP backend that accepts any credentials and
// any mail, so dispatchOneScheduledSend's real Send() call has somewhere to
// successfully submit to.
type acceptingSMTP struct{}

func (acceptingSMTP) NewSession(_ *smtp.Conn) (smtp.Session, error) {
	return acceptingSMTPSession{}, nil
}

type acceptingSMTPSession struct{}

func (acceptingSMTPSession) AuthMechanisms() []string { return []string{sasl.Plain} }
func (acceptingSMTPSession) Auth(string) (sasl.Server, error) {
	return sasl.NewPlainServer(func(_, _, _ string) error { return nil }), nil
}
func (acceptingSMTPSession) Mail(string, *smtp.MailOptions) error { return nil }
func (acceptingSMTPSession) Rcpt(string, *smtp.RcptOptions) error { return nil }
func (acceptingSMTPSession) Data(r io.Reader) error {
	_, err := io.ReadAll(r)
	return err
}
func (acceptingSMTPSession) Reset()        {}
func (acceptingSMTPSession) Logout() error { return nil }

// startMemIMAPAndSMTP launches an in-memory IMAP server (user "username"/
// "password") and a permissive SMTP submission server, returning both
// addresses so a scheduled-send test server can both authenticate a webmail
// session and actually dispatch mail end to end.
func startMemIMAPAndSMTP(t *testing.T) (imapAddr, smtpAddr string) {
	t.Helper()
	imapSrv := imapserver.New(memory.New())
	imapSrv.AllowInsecureAuth = true
	iln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = imapSrv.Serve(iln) }()
	t.Cleanup(func() { _ = imapSrv.Close() })

	smtpSrv := smtp.NewServer(acceptingSMTP{})
	smtpSrv.AllowInsecureAuth = true
	sln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = smtpSrv.Serve(sln) }()
	t.Cleanup(func() { _ = smtpSrv.Close() })

	return iln.Addr().String(), sln.Addr().String()
}

// newScheduledSendMailcowMock combines a fixed mailbox list (matching the
// in-memory IMAP account "username") with appPwMock's app-password mint/
// list/revoke handling, exactly like newSharedMailboxMailcowMock /
// newSSOMailcowMock — ssoWebmailCredential needs both to mint and later
// recover a real app-password.
func newScheduledSendMailcowMock(t *testing.T, username string) string {
	t.Helper()
	appMock := &appPwMock{byName: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/get/mailbox/all", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]mailcow.Mailbox{{Username: username, Active: 1}})
	})
	mux.Handle("/", appMock.handler())
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

// newScheduledSendTestServer builds a full Server with a database and admin
// cipher (so the scheduled-send store is non-nil) and a real in-memory IMAP+
// SMTP backend, so the dispatcher can actually resolve a credential and
// submit mail end to end.
func newScheduledSendTestServer(t *testing.T) (http.Handler, *Server) {
	t.Helper()
	imapAddr, smtpAddr := startMemIMAPAndSMTP(t)

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 11)
	}
	mcURL := newScheduledSendMailcowMock(t, "username")
	cfg := &config.Config{
		MailcowBaseURL: mcURL, MailcowAPIKey: "k",
		AdminUser: "admin", AdminPassword: "pw",
		SessionTTL: time.Hour, WebmailSessionTTL: time.Hour,
		CORSOrigins: []string{"*"}, LoginRateMax: 1000, LoginRateWindow: time.Minute,
		IMAPAddr: imapAddr, SMTPAddr: smtpAddr, MailUseTLS: false,
		DBDriver: "sqlite", DBPath: t.TempDir() + "/scheduled.db",
		AdminEncKey:    key,
		UndoSendWindow: time.Hour, // overridden per-test via the package var where needed
	}
	mc := mailcow.NewClient(cfg.MailcowBaseURL, cfg.MailcowAPIKey, false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	limiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	srv := NewServer(cfg, mc, authn, limiter, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if srv.scheduledSends == nil {
		t.Fatal("expected scheduled sends to be configured (db + admin cipher both present)")
	}
	return srv.Handler(), srv
}

func TestScheduledSendPollIntervalGetter(t *testing.T) {
	if got := ScheduledSendPollInterval(); got != scheduledSendPollInterval {
		t.Errorf("ScheduledSendPollInterval() = %v, want %v", got, scheduledSendPollInterval)
	}
}

func TestSetUndoSendWindowIgnoresNonPositive(t *testing.T) {
	old := undoSendWindow
	defer func() { undoSendWindow = old }()
	undoSendWindow = 42 * time.Second
	SetUndoSendWindow(0)
	if undoSendWindow != 42*time.Second {
		t.Errorf("SetUndoSendWindow(0) should be ignored, got %v", undoSendWindow)
	}
	SetUndoSendWindow(-time.Second)
	if undoSendWindow != 42*time.Second {
		t.Errorf("SetUndoSendWindow(negative) should be ignored, got %v", undoSendWindow)
	}
	SetUndoSendWindow(5 * time.Second)
	if undoSendWindow != 5*time.Second {
		t.Errorf("SetUndoSendWindow(5s) = %v, want 5s", undoSendWindow)
	}
}

func TestScheduledSendDisabledWithoutDB(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{withIMAP: true}) // no DB, no enc key
	tok := webmailToken(t, h)
	if rec := do(h, http.MethodPost, "/api/webmail/scheduled", tok, `{"to":["a@example.com"],"subject":"hi"}`); rec.Code != http.StatusNotImplemented {
		t.Errorf("create without DB = %d, want 501", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/webmail/scheduled", tok, ""); rec.Code != http.StatusNotImplemented {
		t.Errorf("list without DB = %d, want 501", rec.Code)
	}
	if rec := do(h, http.MethodDelete, "/api/webmail/scheduled/1", tok, ""); rec.Code != http.StatusNotImplemented {
		t.Errorf("cancel without DB = %d, want 501", rec.Code)
	}
}

func TestScheduledSendDisabledWithoutEncKey(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{withDB: true, withIMAP: true}) // DB but no enc key
	tok := webmailToken(t, h)
	if rec := do(h, http.MethodPost, "/api/webmail/scheduled", tok, `{"to":["a@example.com"],"subject":"hi"}`); rec.Code != http.StatusNotImplemented {
		t.Errorf("create without enc key = %d, want 501", rec.Code)
	}
}

func TestScheduledSendCreateUnauthorized(t *testing.T) {
	h, _ := newScheduledSendTestServer(t)
	if rec := do(h, http.MethodPost, "/api/webmail/scheduled", "", `{"to":["a@example.com"]}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token = %d, want 401", rec.Code)
	}
}

func TestScheduledSendCreateDefaultsToUndoWindow(t *testing.T) {
	h, srv := newScheduledSendTestServer(t)
	old := undoSendWindow
	undoSendWindow = 30 * time.Second
	defer func() { undoSendWindow = old }()
	tok := webmailToken(t, h)

	before := time.Now()
	rec := do(h, http.MethodPost, "/api/webmail/scheduled", tok, `{"to":["a@example.com"],"subject":"hello","text":"hi"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d, body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		ID          int64  `json:"id"`
		ScheduledAt string `json:"scheduledAt"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ID == 0 {
		t.Error("expected a non-zero id")
	}
	if out.Status != "pending" {
		t.Errorf("status = %q, want pending", out.Status)
	}
	scheduledAt, err := time.Parse(time.RFC3339, out.ScheduledAt)
	if err != nil {
		t.Fatalf("scheduledAt not RFC3339: %v", err)
	}
	// Should land roughly `undoSendWindow` after "before", not immediately
	// and not at some arbitrary distant time.
	delta := scheduledAt.Sub(before)
	if delta < 25*time.Second || delta > 40*time.Second {
		t.Errorf("scheduledAt delta = %v, want ~30s", delta)
	}

	list, err := srv.scheduledSends.ListPending("username")
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(list) != 1 || list[0].ID != out.ID {
		t.Fatalf("ListPending = %+v, want the created row", list)
	}
}

func TestScheduledSendCreateExplicitSendAt(t *testing.T) {
	h, _ := newScheduledSendTestServer(t)
	tok := webmailToken(t, h)

	sendAt := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	body := fmt.Sprintf(`{"to":["a@example.com"],"subject":"hello","sendAt":%q}`, sendAt)
	rec := do(h, http.MethodPost, "/api/webmail/scheduled", tok, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d, body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		ScheduledAt string `json:"scheduledAt"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	got, err := time.Parse(time.RFC3339, out.ScheduledAt)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want, _ := time.Parse(time.RFC3339, sendAt)
	if !got.Equal(want) {
		t.Errorf("scheduledAt = %v, want %v", got, want)
	}
}

func TestScheduledSendCreateRejectsPastSendAt(t *testing.T) {
	h, _ := newScheduledSendTestServer(t)
	tok := webmailToken(t, h)
	sendAt := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	body := fmt.Sprintf(`{"to":["a@example.com"],"subject":"hello","sendAt":%q}`, sendAt)
	if rec := do(h, http.MethodPost, "/api/webmail/scheduled", tok, body); rec.Code != http.StatusBadRequest {
		t.Errorf("past sendAt = %d, want 400", rec.Code)
	}
}

func TestScheduledSendCreateRejectsInvalidSendAt(t *testing.T) {
	h, _ := newScheduledSendTestServer(t)
	tok := webmailToken(t, h)
	if rec := do(h, http.MethodPost, "/api/webmail/scheduled", tok, `{"to":["a@example.com"],"sendAt":"not-a-date"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("invalid sendAt = %d, want 400", rec.Code)
	}
}

func TestScheduledSendCreateRejectsNoRecipients(t *testing.T) {
	h, _ := newScheduledSendTestServer(t)
	tok := webmailToken(t, h)
	if rec := do(h, http.MethodPost, "/api/webmail/scheduled", tok, `{"subject":"hello"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("no recipients = %d, want 400", rec.Code)
	}
}

// The CRLF-injection guard must reject a malicious Subject at creation time
// (400), not only later at dispatch.
func TestScheduledSendCreateRejectsCRLFInjectionInSubject(t *testing.T) {
	h, _ := newScheduledSendTestServer(t)
	tok := webmailToken(t, h)
	body := `{"to":["a@example.com"],"subject":"Hi\r\nBcc: victim@example.com"}`
	rec := do(h, http.MethodPost, "/api/webmail/scheduled", tok, body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("CRLF in subject = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
}

// Same guard, but on a recipient address rather than the subject.
func TestScheduledSendCreateRejectsCRLFInjectionInRecipient(t *testing.T) {
	h, _ := newScheduledSendTestServer(t)
	tok := webmailToken(t, h)
	body := `{"to":["a@example.com\r\nCc: victim@example.com"],"subject":"hi"}`
	rec := do(h, http.MethodPost, "/api/webmail/scheduled", tok, body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("CRLF in recipient = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
}

func TestScheduledSendCreateRejectsBadJSON(t *testing.T) {
	h, _ := newScheduledSendTestServer(t)
	tok := webmailToken(t, h)
	if rec := do(h, http.MethodPost, "/api/webmail/scheduled", tok, `not json`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad json = %d, want 400", rec.Code)
	}
}

func TestScheduledSendListOmitsBodyAndScopesToOwner(t *testing.T) {
	h, srv := newScheduledSendTestServer(t)
	tok := webmailToken(t, h)

	body := `{"to":["a@example.com"],"subject":"hello","text":"secret body","html":"<p>secret</p>","sendAt":"` +
		time.Now().Add(time.Hour).UTC().Format(time.RFC3339) + `"}`
	rec := do(h, http.MethodPost, "/api/webmail/scheduled", tok, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}

	// A row for a different owner must not show up in the list.
	if _, err := srv.scheduledSends.Create("someoneelse@example.com", sampleOutgoing(), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("seed other owner row: %v", err)
	}

	rec = do(h, http.MethodGet, "/api/webmail/scheduled", tok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	if !contains(raw, `"subject":"hello"`) {
		t.Errorf("expected subject in list output, got %s", raw)
	}
	if contains(raw, "secret body") || contains(raw, "secret</p>") {
		t.Errorf("list response must omit text/html bodies, got %s", raw)
	}

	var list []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list should only contain the caller's own row, got %d entries: %+v", len(list), list)
	}
}

func TestScheduledSendCancelHappyPath(t *testing.T) {
	h, _ := newScheduledSendTestServer(t)
	tok := webmailToken(t, h)

	sendAt := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	body := fmt.Sprintf(`{"to":["a@example.com"],"subject":"hi","sendAt":%q}`, sendAt)
	rec := do(h, http.MethodPost, "/api/webmail/scheduled", tok, body)
	var created struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = do(h, http.MethodDelete, fmt.Sprintf("/api/webmail/scheduled/%d", created.ID), tok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel = %d, body=%s", rec.Code, rec.Body.String())
	}

	rec = do(h, http.MethodGet, "/api/webmail/scheduled", tok, "")
	var list []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatalf("canceled row should not be listed, got %+v", list)
	}
}

func TestScheduledSendCancelNotFound(t *testing.T) {
	h, _ := newScheduledSendTestServer(t)
	tok := webmailToken(t, h)
	if rec := do(h, http.MethodDelete, "/api/webmail/scheduled/999999", tok, ""); rec.Code != http.StatusNotFound {
		t.Errorf("cancel nonexistent = %d, want 404", rec.Code)
	}
}

func TestScheduledSendCancelInvalidID(t *testing.T) {
	h, _ := newScheduledSendTestServer(t)
	tok := webmailToken(t, h)
	if rec := do(h, http.MethodDelete, "/api/webmail/scheduled/not-a-number", tok, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("cancel invalid id = %d, want 400", rec.Code)
	}
}

// A second webmail user must not be able to cancel someone else's scheduled
// send — a cross-owner Cancel must report the same 404 as "not found".
func TestScheduledSendCancelWrongOwner(t *testing.T) {
	h, srv := newScheduledSendTestServer(t)
	tok := webmailToken(t, h)

	id, err := srv.scheduledSends.Create("someoneelse@example.com", sampleOutgoing(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if rec := do(h, http.MethodDelete, fmt.Sprintf("/api/webmail/scheduled/%d", id), tok, ""); rec.Code != http.StatusNotFound {
		t.Errorf("cross-owner cancel = %d, want 404", rec.Code)
	}
}

// End-to-end dispatch: a due row gets claimed, sent over the fake SMTP
// backend, and marked 'sent'; it disappears from the pending list. The row
// is seeded directly through the store (rather than the HTTP create
// endpoint, which refuses a past/immediate sendAt) with scheduled_at already
// due, avoiding any dependency on real wall-clock waiting.
func TestDispatchScheduledSendsEndToEnd(t *testing.T) {
	_, srv := newScheduledSendTestServer(t)

	id, err := srv.scheduledSends.Create("username", sampleOutgoing(), time.Now().Add(-time.Second))
	if err != nil {
		t.Fatalf("seed due row: %v", err)
	}

	srv.DispatchScheduledSends()

	list, err := srv.scheduledSends.ListPending("username")
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("dispatched row (id=%d) should no longer be pending, got %+v", id, list)
	}
}

// A row whose dispatch fails (bad IMAP/SMTP credential resolution or a Send
// error) must be marked failed, and must not stop other rows in the same
// batch from being processed.
func TestDispatchScheduledSendsOneFailureDoesNotAbortBatch(t *testing.T) {
	_, srv := newScheduledSendTestServer(t)

	// A row for an owner ssoWebmailCredential cannot resolve (not a known
	// mailbox at all) — this should fail without touching the other row.
	if _, err := srv.scheduledSends.Create("nobody@example.com", sampleOutgoing(), time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("seed failing row: %v", err)
	}
	if _, err := srv.scheduledSends.Create("username", sampleOutgoing(), time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("seed good row: %v", err)
	}

	srv.DispatchScheduledSends()

	// The good row should be gone from pending (sent).
	list, err := srv.scheduledSends.ListPending("username")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("the valid row should have been sent, got %+v", list)
	}
	// The failing row must be marked failed, not stuck in 'sending'.
	failList, err := srv.scheduledSends.ListPending("nobody@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(failList) != 0 {
		t.Fatalf("the failing row should no longer be pending/sending, got %+v", failList)
	}
}

func TestDispatchScheduledSendsNoopWhenDisabled(t *testing.T) {
	h, srv, _ := newAccountTestServer(t, accountTestOpts{withIMAP: true}) // no DB
	_ = h
	if srv.scheduledSends != nil {
		t.Fatal("expected scheduled sends to be nil without a database")
	}
	// Must not panic.
	srv.DispatchScheduledSends()
}

// sampleOutgoing is a minimal, always-valid message for tests that seed rows
// directly through the store rather than via the HTTP API.
func sampleOutgoing() webmail.OutgoingMessage {
	return webmail.OutgoingMessage{To: []string{"someone@example.com"}, Subject: "seeded", Text: "body"}
}

// contains is a tiny strings.Contains alias kept local to this file for
// readability at call sites.
func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }
