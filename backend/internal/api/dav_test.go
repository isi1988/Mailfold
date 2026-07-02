package api

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/backend/memory"
	"github.com/emersion/go-imap/server"
	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
)

func TestDAVVerifier(t *testing.T) {
	calls := 0
	v := newDavVerifier(func(u, p string) error {
		calls++
		if u == "a" && p == "b" {
			return nil
		}
		return errors.New("bad")
	}, time.Hour)

	if !v.ok("a", "b") {
		t.Error("valid credentials should be accepted")
	}
	if !v.ok("a", "b") {
		t.Error("cached credentials should be accepted")
	}
	if calls != 1 {
		t.Errorf("verify called %d times, want 1 (second was cached)", calls)
	}
	if v.ok("a", "wrong") {
		t.Error("wrong password must be rejected")
	}
	if v.ok("x", "y") {
		t.Error("unknown user must be rejected")
	}
}

// davUser is the mailbox the in-memory IMAP backend accepts.
const (
	davUser = "username"
	davPass = "password"
)

// newDAVTestHandler spins up an in-memory IMAP server (for Basic-auth
// verification) and returns a Mailfold HTTP handler wired to it.
func newDAVTestHandler(t *testing.T) http.Handler {
	t.Helper()
	imapSrv := server.New(memory.New())
	imapSrv.AllowInsecureAuth = true
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = imapSrv.Serve(ln) }()
	t.Cleanup(func() { _ = imapSrv.Close() })

	cfg := &config.Config{
		MailcowBaseURL:    "http://mailcow.invalid",
		MailcowAPIKey:     "k",
		AdminUser:         "admin",
		AdminPassword:     "pw",
		SessionTTL:        time.Hour,
		CORSOrigins:       []string{"*"},
		IMAPAddr:          ln.Addr().String(),
		MailUseTLS:        false,
		WebmailSessionTTL: time.Hour,
		DBDriver:          "sqlite",
		DBPath:            t.TempDir() + "/dav.db",
	}
	mc := mailcow.NewClient(cfg.MailcowBaseURL, cfg.MailcowAPIKey, false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, time.Hour)
	limiter := ratelimit.New(0, time.Minute)
	return NewServer(cfg, mc, authn, limiter, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
}

func TestCardDAVHTTP(t *testing.T) {
	h := newDAVTestHandler(t)

	// Unauthenticated DAV request is rejected.
	req := httptest.NewRequest("PROPFIND", "/dav/carddav/username/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("PROPFIND without auth = %d, want 401", rec.Code)
	}

	// PUT a vCard.
	vc := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:t1\r\nFN:Test User\r\nEND:VCARD\r\n"
	put := httptest.NewRequest(http.MethodPut, "/dav/carddav/username/default/t1.vcf", strings.NewReader(vc))
	put.SetBasicAuth(davUser, davPass)
	put.Header.Set("Content-Type", "text/vcard")
	putRec := httptest.NewRecorder()
	h.ServeHTTP(putRec, put)
	if putRec.Code != http.StatusCreated && putRec.Code != http.StatusNoContent && putRec.Code != http.StatusOK {
		t.Fatalf("PUT vCard = %d, body=%s", putRec.Code, putRec.Body.String())
	}

	// GET it back.
	get := httptest.NewRequest(http.MethodGet, "/dav/carddav/username/default/t1.vcf", nil)
	get.SetBasicAuth(davUser, davPass)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, get)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET vCard = %d", getRec.Code)
	}
	if !strings.Contains(getRec.Body.String(), "Test User") {
		t.Errorf("GET body missing contact: %s", getRec.Body.String())
	}
}

func TestCalDAVHTTP(t *testing.T) {
	h := newDAVTestHandler(t)

	// Unauthenticated DAV request is rejected.
	req := httptest.NewRequest("PROPFIND", "/dav/caldav/username/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("PROPFIND without auth = %d, want 401", rec.Code)
	}

	// PUT an iCalendar event.
	ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Mailfold//Test//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:e1\r\nDTSTAMP:20260702T090000Z\r\n" +
		"DTSTART:20260702T100000Z\r\nDTEND:20260702T110000Z\r\nSUMMARY:Standup\r\n" +
		"END:VEVENT\r\nEND:VCALENDAR\r\n"
	put := httptest.NewRequest(http.MethodPut, "/dav/caldav/username/default/e1.ics", strings.NewReader(ics))
	put.SetBasicAuth(davUser, davPass)
	put.Header.Set("Content-Type", "text/calendar")
	putRec := httptest.NewRecorder()
	h.ServeHTTP(putRec, put)
	if putRec.Code != http.StatusCreated && putRec.Code != http.StatusNoContent && putRec.Code != http.StatusOK {
		t.Fatalf("PUT iCal = %d, body=%s", putRec.Code, putRec.Body.String())
	}

	// GET it back.
	get := httptest.NewRequest(http.MethodGet, "/dav/caldav/username/default/e1.ics", nil)
	get.SetBasicAuth(davUser, davPass)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, get)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET iCal = %d", getRec.Code)
	}
	if !strings.Contains(getRec.Body.String(), "Standup") {
		t.Errorf("GET body missing event: %s", getRec.Body.String())
	}
}
