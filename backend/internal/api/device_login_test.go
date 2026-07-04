package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/apikey"
	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
)

func TestDeviceLoginMintsWebmailSession(t *testing.T) {
	srv, h := newAPIKeyServer(t, 120)
	tok := insertKey(t, srv, apikey.DefaultScopes())

	rec := doKey(t, h, http.MethodPost, "/api/auth/device-login", "", `{"key":"`+tok+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("device-login = %d, body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Token string `json:"token"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Email != "username" || out.Token == "" {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
	if _, ok := srv.webmailSessions.Get(out.Token); !ok {
		t.Fatal("minted token is not a valid webmail session")
	}
}

func TestDeviceLoginRejectsBadOrRevokedKey(t *testing.T) {
	srv, h := newAPIKeyServer(t, 120)

	if rec := doKey(t, h, http.MethodPost, "/api/auth/device-login", "", `{"key":"nonsense"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("malformed key = %d, want 401", rec.Code)
	}

	tok := insertKey(t, srv, apikey.DefaultScopes())
	recs, err := srv.apikeyStore.List("")
	if err != nil || len(recs) != 1 {
		t.Fatalf("expected exactly one key, got %d, err=%v", len(recs), err)
	}
	if _, err := srv.apikeyStore.Revoke(recs[0].ID, recs[0].Created); err != nil {
		t.Fatal(err)
	}
	if rec := doKey(t, h, http.MethodPost, "/api/auth/device-login", "", `{"key":"`+tok+`"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked key = %d, want 401", rec.Code)
	}
}

func TestDeviceLoginDisabledWithoutAPIKeyStore(t *testing.T) {
	cfg := &config.Config{
		MailcowBaseURL:  mockMailcow(t, 0, "").URL,
		MailcowAPIKey:   "k",
		AdminUser:       "admin",
		AdminPassword:   "pw",
		SessionTTL:      time.Hour,
		CORSOrigins:     []string{"*"},
		LoginRateMax:    1000,
		LoginRateWindow: time.Minute,
	}
	mc := mailcow.NewClient(cfg.MailcowBaseURL, cfg.MailcowAPIKey, false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	limiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	h := NewServer(cfg, mc, authn, limiter, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()

	rec := doKey(t, h, http.MethodPost, "/api/auth/device-login", "", `{"key":"mf_live_x_y"}`)
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("device-login without store = %d, want 501", rec.Code)
	}
}
