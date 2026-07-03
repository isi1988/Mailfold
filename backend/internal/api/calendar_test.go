package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
)

// newAPIWithDAV builds a server with both a webmail IMAP and a SQLite DAV store,
// so the webmail calendar endpoints can be exercised end to end.
func newAPIWithDAV(t *testing.T, mcURL, imapAddr string) http.Handler {
	t.Helper()
	cfg := &config.Config{
		MailcowBaseURL: mcURL, MailcowAPIKey: "k",
		AdminUser: "admin", AdminPassword: "pw", SessionTTL: time.Hour,
		CORSOrigins: []string{"*"}, LoginRateMax: 100, LoginRateWindow: time.Minute,
		MaxBodyBytes: 1 << 20,
		IMAPAddr:     imapAddr, MailUseTLS: false, WebmailSessionTTL: time.Hour,
		DBDriver: "sqlite", DBPath: filepath.Join(t.TempDir(), "dav.db"),
	}
	mc := mailcow.NewClient(mcURL, "k", false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	limiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(cfg, mc, authn, limiter, logger).Handler()
}

func TestWebmailCalendarCRUD(t *testing.T) {
	h := newAPIWithDAV(t, mockMailcow(t, 0, "").URL, startMemIMAP(t))
	wt := webmailToken(t, h)

	// Empty to start.
	rec := do(h, http.MethodGet, "/api/webmail/calendar/events", wt, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var events []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &events)
	if len(events) != 0 {
		t.Fatalf("expected no events, got %d", len(events))
	}

	// Create.
	body := `{"summary":"Team sync","start":"2026-07-10T09:00:00Z","end":"2026-07-10T10:00:00Z","location":"Room 1"}`
	rec = do(h, http.MethodPost, "/api/webmail/calendar/events", wt, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		UID string `json:"uid"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.UID == "" {
		t.Fatal("create returned no uid")
	}

	// List now has the event with its fields parsed back.
	rec = do(h, http.MethodGet, "/api/webmail/calendar/events", wt, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &events)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0]["summary"] != "Team sync" {
		t.Errorf("summary = %v", events[0]["summary"])
	}

	// Missing summary -> 400.
	if r2 := do(h, http.MethodPost, "/api/webmail/calendar/events", wt, `{"summary":""}`); r2.Code != http.StatusBadRequest {
		t.Fatalf("empty summary: want 400, got %d", r2.Code)
	}

	// Delete.
	if r3 := do(h, http.MethodDelete, "/api/webmail/calendar/events/"+created.UID, wt, ""); r3.Code != http.StatusOK {
		t.Fatalf("delete: %d", r3.Code)
	}
	rec = do(h, http.MethodGet, "/api/webmail/calendar/events", wt, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &events)
	if len(events) != 0 {
		t.Fatalf("expected 0 after delete, got %d", len(events))
	}

	// Unauthenticated.
	if r4 := do(h, http.MethodGet, "/api/webmail/calendar/events", "", ""); r4.Code != http.StatusUnauthorized {
		t.Fatalf("no token: want 401, got %d", r4.Code)
	}
}
