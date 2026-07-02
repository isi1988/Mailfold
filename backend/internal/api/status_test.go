package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
)

// newAPIWithServerName builds a server whose config carries the given public
// mail-server name, so the /api/status/server handler can be exercised.
func newAPIWithServerName(t *testing.T, mcURL, serverName string) http.Handler {
	t.Helper()
	cfg := &config.Config{
		MailcowBaseURL:  mcURL,
		MailcowAPIKey:   "k",
		AdminUser:       "admin",
		AdminPassword:   "pw",
		SessionTTL:      time.Hour,
		CORSOrigins:     []string{"*"},
		LoginRateMax:    100,
		LoginRateWindow: time.Minute,
		MaxBodyBytes:    1 << 20,
		ServerName:      serverName,
	}
	mc := mailcow.NewClient(mcURL, "k", false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	limiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(cfg, mc, authn, limiter, logger).Handler()
}

func TestStatusServerName(t *testing.T) {
	h := newAPIWithServerName(t, mockMailcow(t, 0, "").URL, "mail.test.example")

	// Unauthenticated callers are rejected.
	if rec := do(h, http.MethodGet, "/api/status/server", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}

	token := loginToken(t, h)
	rec := do(h, http.MethodGet, "/api/status/server", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var out struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Name != "mail.test.example" {
		t.Fatalf("name=%q, want mail.test.example", out.Name)
	}
}

func TestStatusServerNameEmpty(t *testing.T) {
	h := newAPIWithServerName(t, mockMailcow(t, 0, "").URL, "")
	token := loginToken(t, h)
	rec := do(h, http.MethodGet, "/api/status/server", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var out struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Name != "" {
		t.Fatalf("name=%q, want empty", out.Name)
	}
}
