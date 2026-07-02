package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
)

// TestFrontendServedWithoutRouteConflict guards against the catch-all SPA route
// conflicting with the method-less /dav/ subtree under Go 1.22's ServeMux (which
// panicked at Handler() build time once a frontend build was present).
func TestFrontendServedWithoutRouteConflict(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/index.html", []byte("<!doctype html><title>Mailfold</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		MailcowBaseURL:    "http://mailcow.invalid",
		MailcowAPIKey:     "k",
		AdminUser:         "admin",
		AdminPassword:     "pw",
		SessionTTL:        time.Hour,
		CORSOrigins:       []string{"*"},
		WebmailSessionTTL: time.Hour,
		DBDriver:          "sqlite",
		DBPath:            dir + "/db.sqlite", // registers the /dav/ routes
		FrontendDir:       dir,                // registers the "/" catch-all
	}
	mc := mailcow.NewClient(cfg.MailcowBaseURL, cfg.MailcowAPIKey, false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, time.Hour)

	// Handler() must not panic building the mux.
	h := NewServer(cfg, mc, authn, ratelimit.New(0, time.Minute), slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()

	// A client-side route falls back to index.html.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/mailboxes", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Mailfold") {
		t.Fatalf("SPA fallback = %d, body=%q", rec.Code, rec.Body.String())
	}

	// The API and DAV routes still win over the catch-all.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("/api/health = %d, want 200", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("PROPFIND", "/dav/carddav/user/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("DAV route = %d, want 401 (not the SPA)", rec.Code)
	}
}
