package api

import (
	"context"
	"encoding/json"
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

// startMemIMAP launches an in-memory IMAP server (user "username"/"password"
// with a sample INBOX) and returns its address.
func startMemIMAP(t *testing.T) string {
	t.Helper()
	s := server.New(memory.New())
	s.AllowInsecureAuth = true
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = s.Serve(ln) }()
	t.Cleanup(func() { _ = s.Close() })
	return ln.Addr().String()
}

// newAPIWithIMAP builds a server whose webmail client points at a plaintext IMAP
// address, so the webmail login and events endpoints can be exercised.
func newAPIWithIMAP(t *testing.T, mcURL, imapAddr string) http.Handler {
	t.Helper()
	cfg := &config.Config{
		MailcowBaseURL:    mcURL,
		MailcowAPIKey:     "k",
		AdminUser:         "admin",
		AdminPassword:     "pw",
		SessionTTL:        time.Hour,
		CORSOrigins:       []string{"*"},
		LoginRateMax:      100,
		LoginRateWindow:   time.Minute,
		MaxBodyBytes:      1 << 20,
		IMAPAddr:          imapAddr,
		MailUseTLS:        false,
		WebmailSessionTTL: time.Hour,
	}
	mc := mailcow.NewClient(mcURL, "k", false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	limiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(cfg, mc, authn, limiter, logger).Handler()
}

func webmailToken(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := do(h, http.MethodPost, "/api/webmail/login", "", `{"email":"username","password":"password"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("webmail login code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Token == "" {
		t.Fatal("webmail login returned no token")
	}
	return out.Token
}

func TestWebmailEventsUnauthorized(t *testing.T) {
	h := newAPIWithIMAP(t, mockMailcow(t, 0, "").URL, startMemIMAP(t))
	if rec := do(h, http.MethodGet, "/api/webmail/events", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: want 401, got %d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/webmail/events?token=bogus", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad token: want 401, got %d", rec.Code)
	}
}

func TestWebmailEventsStream(t *testing.T) {
	h := newAPIWithIMAP(t, mockMailcow(t, 0, "").URL, startMemIMAP(t))
	wt := webmailToken(t, h)

	old := webmailEventInterval
	webmailEventInterval = 25 * time.Millisecond
	defer func() { webmailEventInterval = old }()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/webmail/events?token="+wt, nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	go func() {
		time.Sleep(160 * time.Millisecond)
		cancel()
	}()
	h.ServeHTTP(rec, req)

	body := rec.Body.String()
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("content-type = %q, want text/event-stream", ct)
	}
	if !strings.Contains(body, ": connected") {
		t.Fatalf("stream missing connected marker: %q", body)
	}
	// With no new mail arriving, at least one poll tick must have produced a
	// keepalive ping.
	if !strings.Contains(body, ": ping") {
		t.Fatalf("stream missing keepalive ping (no tick fired): %q", body)
	}
}
