package api

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
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
		MaxBodyBytes: 25 << 20,
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

// TestWebmailCalendarRichEvent exercises guests, repeat, reminder and file
// attachments end to end, including downloading an attachment back.
func TestWebmailCalendarRichEvent(t *testing.T) {
	h := newAPIWithDAV(t, mockMailcow(t, 0, "").URL, startMemIMAP(t))
	wt := webmailToken(t, h)

	payload := createEventRequest{
		Summary:  "Roadmap review",
		Start:    time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC),
		End:      time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC),
		Calendar: "Work",
		Guests:   []string{"a@acme.io", "b@acme.io"},
		Repeat:   "WEEKLY",
		Reminder: 30,
		Attachments: []eventAttachment{{
			Filename: "agenda.txt",
			Mime:     "text/plain",
			Data:     base64.StdEncoding.EncodeToString([]byte("hello world")),
		}},
	}
	raw, _ := json.Marshal(payload)
	rec := do(h, http.MethodPost, "/api/webmail/calendar/events", wt, string(raw))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		UID string `json:"uid"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// Fields round-trip through the CalDAV store.
	rec = do(h, http.MethodGet, "/api/webmail/calendar/events", wt, "")
	var events []calendarEvent
	_ = json.Unmarshal(rec.Body.Bytes(), &events)
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Calendar != "Work" {
		t.Errorf("calendar = %q", ev.Calendar)
	}
	if len(ev.Guests) != 2 || ev.Guests[0] != "a@acme.io" {
		t.Errorf("guests = %v", ev.Guests)
	}
	if ev.Repeat != "WEEKLY" {
		t.Errorf("repeat = %q", ev.Repeat)
	}
	if ev.Reminder != 30 {
		t.Errorf("reminder = %d", ev.Reminder)
	}
	if len(ev.Attachments) != 1 || ev.Attachments[0].Filename != "agenda.txt" {
		t.Fatalf("attachments = %+v", ev.Attachments)
	}
	if ev.Attachments[0].Size != len("hello world") {
		t.Errorf("attachment size = %d", ev.Attachments[0].Size)
	}
	if ev.Attachments[0].Data != "" {
		t.Error("list should not carry attachment data")
	}

	// Download the attachment back.
	dl := do(h, http.MethodGet, "/api/webmail/calendar/events/"+created.UID+"/attachments/0", wt, "")
	if dl.Code != http.StatusOK {
		t.Fatalf("download: %d %s", dl.Code, dl.Body.String())
	}
	if dl.Body.String() != "hello world" {
		t.Errorf("download body = %q", dl.Body.String())
	}
	if cd := dl.Header().Get("Content-Disposition"); !strings.Contains(cd, "agenda.txt") {
		t.Errorf("content-disposition = %q", cd)
	}

	// Out-of-range attachment index -> 404.
	if r := do(h, http.MethodGet, "/api/webmail/calendar/events/"+created.UID+"/attachments/5", wt, ""); r.Code != http.StatusNotFound {
		t.Errorf("bad index: want 404, got %d", r.Code)
	}

	// Oversized attachments are rejected.
	big := createEventRequest{Summary: "big", Attachments: []eventAttachment{{Filename: "x", Data: strings.Repeat("A", 14<<20)}}}
	rawBig, _ := json.Marshal(big)
	if r := do(h, http.MethodPost, "/api/webmail/calendar/events", wt, string(rawBig)); r.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized: want 413, got %d", r.Code)
	}
}

// TestWebmailCalendarAllDayAndAttachment covers all-day events, a daily repeat,
// filename sanitisation and the attachment error paths.
func TestWebmailCalendarAllDayAndAttachment(t *testing.T) {
	h := newAPIWithDAV(t, mockMailcow(t, 0, "").URL, startMemIMAP(t))
	wt := webmailToken(t, h)

	payload := createEventRequest{
		Summary: "Cabin weekend",
		Start:   time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
		End:     time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC),
		AllDay:  true,
		Repeat:  "DAILY",
		Attachments: []eventAttachment{{
			Filename: `../my "trip".pdf`,
			Mime:     "application/pdf",
			Data:     base64.StdEncoding.EncodeToString([]byte("%PDF-1.4")),
		}},
	}
	raw, _ := json.Marshal(payload)
	rec := do(h, http.MethodPost, "/api/webmail/calendar/events", wt, string(raw))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		UID string `json:"uid"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = do(h, http.MethodGet, "/api/webmail/calendar/events", wt, "")
	var events []calendarEvent
	_ = json.Unmarshal(rec.Body.Bytes(), &events)
	if len(events) != 1 || !events[0].AllDay {
		t.Fatalf("expected one all-day event, got %+v", events)
	}
	if events[0].Repeat != "DAILY" {
		t.Errorf("repeat = %q", events[0].Repeat)
	}

	// Downloaded filename is sanitised (no quotes or slashes).
	dl := do(h, http.MethodGet, "/api/webmail/calendar/events/"+created.UID+"/attachments/0", wt, "")
	if dl.Code != http.StatusOK || dl.Body.String() != "%PDF-1.4" {
		t.Fatalf("download: %d %q", dl.Code, dl.Body.String())
	}
	cd := dl.Header().Get("Content-Disposition")
	if strings.Contains(cd, "/") || !strings.Contains(cd, "trip") {
		t.Errorf("content-disposition not sanitised: %q", cd)
	}

	// Non-numeric index -> 400.
	if r := do(h, http.MethodGet, "/api/webmail/calendar/events/"+created.UID+"/attachments/x", wt, ""); r.Code != http.StatusBadRequest {
		t.Errorf("bad index: want 400, got %d", r.Code)
	}
	// Unknown event -> 404.
	if r := do(h, http.MethodGet, "/api/webmail/calendar/events/nope/attachments/0", wt, ""); r.Code != http.StatusNotFound {
		t.Errorf("unknown event: want 404, got %d", r.Code)
	}
}
