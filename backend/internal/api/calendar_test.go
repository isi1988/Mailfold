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
		Summary:     "Roadmap review, Q3",
		Description: "line one\nline two; and, more",
		Start:       time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC),
		End:         time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC),
		Calendar:    "Work",
		Guests:      []string{"a@acme.io", "b@acme.io"},
		Repeat:      "WEEKLY",
		Reminder:    30,
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
	if ev.Summary != "Roadmap review, Q3" {
		t.Errorf("summary not decoded: %q", ev.Summary)
	}
	if ev.Description != "line one\nline two; and, more" {
		t.Errorf("description not decoded: %q", ev.Description)
	}
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

// TestWebmailCalendarRsvp covers the owner's RSVP: it round-trips on create,
// keeps the owner out of the guest list, and can be patched in place.
func TestWebmailCalendarRsvp(t *testing.T) {
	h := newAPIWithDAV(t, mockMailcow(t, 0, "").URL, startMemIMAP(t))
	wt := webmailToken(t, h)

	body := `{"summary":"Roadmap","start":"2026-07-12T09:00:00Z","end":"2026-07-12T10:00:00Z","guests":["a@acme.io","b@acme.io"],"rsvp":"yes"}`
	rec := do(h, http.MethodPost, "/api/webmail/calendar/events", wt, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		UID string `json:"uid"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	get := func() calendarEvent {
		r := do(h, http.MethodGet, "/api/webmail/calendar/events", wt, "")
		var evs []calendarEvent
		_ = json.Unmarshal(r.Body.Bytes(), &evs)
		if len(evs) != 1 {
			t.Fatalf("want 1 event, got %d", len(evs))
		}
		return evs[0]
	}

	ev := get()
	if ev.Rsvp != "yes" {
		t.Errorf("rsvp on create = %q, want yes", ev.Rsvp)
	}
	if len(ev.Guests) != 2 {
		t.Errorf("owner leaked into guests: %v", ev.Guests)
	}

	// Patch the response.
	pr := do(h, http.MethodPatch, "/api/webmail/calendar/events/"+created.UID+"/rsvp", wt, `{"rsvp":"maybe"}`)
	if pr.Code != http.StatusOK {
		t.Fatalf("rsvp patch: %d %s", pr.Code, pr.Body.String())
	}
	if got := get().Rsvp; got != "maybe" {
		t.Errorf("rsvp after patch = %q, want maybe", got)
	}
	if do(h, http.MethodPatch, "/api/webmail/calendar/events/"+created.UID+"/rsvp", wt, `{"rsvp":"no"}`); get().Rsvp != "no" {
		t.Errorf("rsvp declined round-trip failed")
	}

	// Unknown event -> 404.
	if r := do(h, http.MethodPatch, "/api/webmail/calendar/events/missing/rsvp", wt, `{"rsvp":"yes"}`); r.Code != http.StatusNotFound {
		t.Errorf("rsvp on missing event: want 404, got %d", r.Code)
	}
}

// TestWebmailCalendarUpdate checks that an edit/move rewrites the editable
// fields while preserving attachments and the owner's RSVP.
func TestWebmailCalendarUpdate(t *testing.T) {
	h := newAPIWithDAV(t, mockMailcow(t, 0, "").URL, startMemIMAP(t))
	wt := webmailToken(t, h)

	create := createEventRequest{
		Summary: "Draft", Start: time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC),
		Calendar: "Work", Rsvp: "yes",
		Attachments: []eventAttachment{{Filename: "spec.txt", Mime: "text/plain", Data: base64.StdEncoding.EncodeToString([]byte("keep me"))}},
	}
	raw, _ := json.Marshal(create)
	rec := do(h, http.MethodPost, "/api/webmail/calendar/events", wt, string(raw))
	var created struct {
		UID string `json:"uid"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// Move + rename; send no attachments and no rsvp — both must survive.
	upd := createEventRequest{Summary: "Renamed", Start: time.Date(2026, 7, 13, 14, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC), Calendar: "Team"}
	uraw, _ := json.Marshal(upd)
	if r := do(h, http.MethodPut, "/api/webmail/calendar/events/"+created.UID, wt, string(uraw)); r.Code != http.StatusOK {
		t.Fatalf("update: %d %s", r.Code, r.Body.String())
	}

	rec = do(h, http.MethodGet, "/api/webmail/calendar/events", wt, "")
	var evs []calendarEvent
	_ = json.Unmarshal(rec.Body.Bytes(), &evs)
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d", len(evs))
	}
	ev := evs[0]
	if ev.Summary != "Renamed" || ev.Calendar != "Team" {
		t.Errorf("edit not applied: %q / %q", ev.Summary, ev.Calendar)
	}
	if !ev.Start.Equal(time.Date(2026, 7, 13, 14, 0, 0, 0, time.UTC)) {
		t.Errorf("move not applied: %v", ev.Start)
	}
	if ev.Rsvp != "yes" {
		t.Errorf("rsvp not preserved: %q", ev.Rsvp)
	}
	if len(ev.Attachments) != 1 || ev.Attachments[0].Filename != "spec.txt" {
		t.Fatalf("attachments not preserved: %+v", ev.Attachments)
	}
	dl := do(h, http.MethodGet, "/api/webmail/calendar/events/"+created.UID+"/attachments/0", wt, "")
	if dl.Body.String() != "keep me" {
		t.Errorf("attachment data lost: %q", dl.Body.String())
	}

	// Unknown event -> 404.
	if r := do(h, http.MethodPut, "/api/webmail/calendar/events/nope", wt, string(uraw)); r.Code != http.StatusNotFound {
		t.Errorf("update missing: want 404, got %d", r.Code)
	}
}

// TestEditEventPreservesRichProps verifies an in-place edit keeps recurrence
// detail, foreign alarms, the organiser and guest parameters that the webmail
// form cannot model.
func TestEditEventPreservesRichProps(t *testing.T) {
	const raw = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//ext//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:rich@ext\r\nSUMMARY:Standup\r\nLOCATION:Old room\r\n" +
		"DTSTART:20260706T090000Z\r\nDTEND:20260706T093000Z\r\n" +
		"RRULE:FREQ=WEEKLY;BYDAY=MO,WE,FR;COUNT=10;INTERVAL=2\r\n" +
		"ORGANIZER:mailto:boss@acme.io\r\n" +
		"ATTENDEE;PARTSTAT=ACCEPTED:mailto:me@acme.io\r\n" +
		"ATTENDEE;CN=Kai;PARTSTAT=TENTATIVE:mailto:kai@acme.io\r\n" +
		"BEGIN:VALARM\r\nACTION:EMAIL\r\nTRIGGER:-P1D\r\nDESCRIPTION:ping\r\nEND:VALARM\r\n" +
		"END:VEVENT\r\nEND:VCALENDAR\r\n"

	// Edit only the summary/location/time; keep WEEKLY, keep guest kai.
	req := createEventRequest{
		Summary: "Standup (moved)", Location: "New room", Calendar: "Work", Repeat: "WEEKLY",
		Start: time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 6, 10, 30, 0, 0, time.UTC),
		Guests: []string{"kai@acme.io", "new@acme.io"},
	}
	out, ok := editEvent(raw, req, "me@acme.io")
	if !ok {
		t.Fatal("editEvent failed")
	}
	for _, want := range []string{
		"SUMMARY:Standup (moved)", "LOCATION:New room",
		"BYDAY=MO,WE,FR", "COUNT=10", "INTERVAL=2", // recurrence detail preserved (freq unchanged)
		"ACTION:EMAIL", "TRIGGER:-P1D", // foreign alarm preserved
		"ORGANIZER:mailto:boss@acme.io", // organiser preserved
		"PARTSTAT=ACCEPTED", "CN=Kai",   // owner RSVP + guest params preserved
		"mailto:new@acme.io", // new guest added
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output; got:\n%s", want, out)
		}
	}

	// Parse back: owner not a guest, WEEKLY reported, times moved.
	ev, _ := parseEvent(out, "me@acme.io")
	if ev.Repeat != "WEEKLY" || ev.Summary != "Standup (moved)" {
		t.Errorf("parsed = %+v", ev)
	}
	if len(ev.Guests) != 2 {
		t.Errorf("guests = %v", ev.Guests)
	}

	// Changing the frequency replaces the rule; clearing it removes the rule.
	if o, _ := editEvent(raw, createEventRequest{Summary: "x", Repeat: "MONTHLY", Start: req.Start, End: req.End}, "me@acme.io"); strings.Contains(o, "BYDAY") || !strings.Contains(o, "FREQ=MONTHLY") {
		t.Error("changing frequency should replace the rule")
	}
	if o, _ := editEvent(raw, createEventRequest{Summary: "x", Repeat: "", Start: req.Start, End: req.End}, "me@acme.io"); strings.Contains(o, "RRULE") {
		t.Error("clearing repeat should drop the rule")
	}

	// Setting a reminder keeps the foreign EMAIL alarm and adds our display alarm.
	if o, _ := editEvent(raw, createEventRequest{Summary: "x", Repeat: "WEEKLY", Reminder: 15, Start: req.Start, End: req.End}, "me@acme.io"); !strings.Contains(o, "ACTION:EMAIL") || !strings.Contains(o, "-PT15M") {
		t.Error("changing reminder should keep the EMAIL alarm and add -PT15M")
	}

	// An event whose only alarm is our simple reminder: editing swaps its trigger.
	const simple = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//x//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:s@x\r\nSUMMARY:s\r\nDTSTART:20260706T090000Z\r\nDTEND:20260706T093000Z\r\n" +
		"BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT10M\r\nDESCRIPTION:s\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	o3, _ := editEvent(simple, createEventRequest{Summary: "s", Reminder: 30, Start: req.Start, End: req.End}, "me@x")
	if strings.Contains(o3, "-PT10M") || !strings.Contains(o3, "-PT30M") || strings.Count(o3, "BEGIN:VALARM") != 1 {
		t.Errorf("simple reminder should be replaced 10->30 with one alarm:\n%s", o3)
	}
}

func TestRsvpPartstatMapping(t *testing.T) {
	cases := []struct{ rsvp, partstat string }{
		{"yes", "ACCEPTED"}, {"maybe", "TENTATIVE"}, {"no", "DECLINED"}, {"none", "NEEDS-ACTION"}, {"bogus", "NEEDS-ACTION"},
	}
	for _, c := range cases {
		if got := rsvpToPartstat(c.rsvp); got != c.partstat {
			t.Errorf("rsvpToPartstat(%q) = %q, want %q", c.rsvp, got, c.partstat)
		}
	}
	back := map[string]string{"ACCEPTED": "yes", "TENTATIVE": "maybe", "DECLINED": "no", "NEEDS-ACTION": "none", "X-WEIRD": "none"}
	for ps, want := range back {
		if got := partstatToRsvp(ps); got != want {
			t.Errorf("partstatToRsvp(%q) = %q, want %q", ps, got, want)
		}
	}
}
