package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
)

// mockMailcow returns a mock mailcow server. When body is non-empty it is
// returned for every request; otherwise success/list JSON is synthesized. A
// non-zero status is written before the body.
func mockMailcow(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if status != 0 {
			w.WriteHeader(status)
		}
		if body != "" {
			_, _ = io.WriteString(w, body)
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/add/"),
			strings.HasPrefix(r.URL.Path, "/api/v1/edit/"),
			strings.HasPrefix(r.URL.Path, "/api/v1/delete/"):
			_, _ = io.WriteString(w, `[{"type":"success","msg":["ok"]}]`)
		default:
			_, _ = io.WriteString(w, `[]`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newAPI(t *testing.T, mcURL string, origins []string) http.Handler {
	t.Helper()
	cfg := &config.Config{
		MailcowBaseURL: mcURL,
		MailcowAPIKey:  "k",
		AdminUser:      "admin",
		AdminPassword:  "pw",
		SessionTTL:     time.Hour,
		CORSOrigins:    origins,
	}
	mc := mailcow.NewClient(mcURL, "k", false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(cfg, mc, authn, logger).Handler()
}

func loginToken(t *testing.T, h http.Handler) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"user": "admin", "password": "pw"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login code=%d", rec.Code)
	}
	var out struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Token == "" {
		t.Fatal("login returned no token")
	}
	return out.Token
}

func do(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthAndAuthFlow(t *testing.T) {
	h := newAPI(t, mockMailcow(t, 0, "").URL, []string{"*"})

	if rec := do(h, http.MethodGet, "/api/health", "", ""); rec.Code != http.StatusOK {
		t.Fatalf("health=%d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/domains", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 unauthorized, got %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/auth/login", "", `{"user":"admin","password":"nope"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login=%d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/auth/login", "", "{bad"); rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed login=%d", rec.Code)
	}

	token := loginToken(t, h)
	if rec := do(h, http.MethodGet, "/api/auth/me", token, ""); rec.Code != http.StatusOK {
		t.Fatalf("me=%d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/auth/logout", token, ""); rec.Code != http.StatusOK {
		t.Fatalf("logout=%d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/domains", token, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 after logout, got %d", rec.Code)
	}
}

func TestResourceEndpoints(t *testing.T) {
	h := newAPI(t, mockMailcow(t, 0, "").URL, []string{"*"})
	token := loginToken(t, h)

	attr := `{"domain":"x"}`
	edit := `{"items":["a"],"attr":{"active":"1"}}`
	del := `{"items":["a"]}`
	cases := []struct {
		method, path, body string
		want               int
	}{
		{http.MethodGet, "/api/domains", "", 200},
		{http.MethodPost, "/api/domains", attr, 200},
		{http.MethodPut, "/api/domains", edit, 200},
		{http.MethodDelete, "/api/domains", del, 200},
		{http.MethodGet, "/api/mailboxes", "", 200},
		{http.MethodPost, "/api/mailboxes", attr, 200},
		{http.MethodPut, "/api/mailboxes", edit, 200},
		{http.MethodDelete, "/api/mailboxes", del, 200},
		{http.MethodGet, "/api/aliases", "", 200},
		{http.MethodPost, "/api/aliases", attr, 200},
		{http.MethodPut, "/api/aliases", edit, 200},
		{http.MethodDelete, "/api/aliases", del, 200},
		{http.MethodGet, "/api/syncjobs", "", 200},
		{http.MethodPost, "/api/syncjobs", attr, 200},
		{http.MethodPut, "/api/syncjobs", edit, 200},
		{http.MethodDelete, "/api/syncjobs", del, 200},
		{http.MethodGet, "/api/dkim/example.com", "", 200},
		{http.MethodPost, "/api/dkim", attr, 200},
		{http.MethodDelete, "/api/dkim", del, 200},
		{http.MethodGet, "/api/queue", "", 200},
		{http.MethodPost, "/api/queue/flush", "", 200},
		{http.MethodGet, "/api/logs/postfix", "", 200},
		{http.MethodGet, "/api/logs/postfix?count=5", "", 200},
		{http.MethodGet, "/api/logs/postfix?count=0", "", 200},
		{http.MethodGet, "/api/logs/postfix?count=99999", "", 200},
		{http.MethodGet, "/api/logs/unknownsvc", "", 400},
		{http.MethodGet, "/api/fail2ban", "", 200},
		{http.MethodPut, "/api/fail2ban", attr, 200},
		{http.MethodGet, "/api/quarantine", "", 200},
		{http.MethodDelete, "/api/quarantine", del, 200},
		{http.MethodGet, "/api/policy/allow/example.com", "", 200},
		{http.MethodGet, "/api/policy/deny/example.com", "", 200},
		{http.MethodPost, "/api/policy", attr, 200},
		{http.MethodDelete, "/api/policy", del, 200},
		{http.MethodGet, "/api/status/containers", "", 200},
		{http.MethodGet, "/api/status/version", "", 200},
		{http.MethodGet, "/api/status/vmail", "", 200},
	}
	for _, c := range cases {
		rec := do(h, c.method, c.path, token, c.body)
		if rec.Code != c.want {
			t.Errorf("%s %s: got %d want %d", c.method, c.path, rec.Code, c.want)
		}
	}
}

func TestBadRequestAndUpstreamErrors(t *testing.T) {
	// Malformed body -> 400.
	h := newAPI(t, mockMailcow(t, 0, "").URL, []string{"*"})
	token := loginToken(t, h)
	if rec := do(h, http.MethodPost, "/api/domains", token, "{bad"); rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}

	// Upstream HTTP 500 -> 502 across every handler shape.
	h2 := newAPI(t, mockMailcow(t, http.StatusInternalServerError, "").URL, []string{"*"})
	t2 := loginToken(t, h2)
	upstream := []struct{ method, path, body string }{
		{http.MethodGet, "/api/domains", ""},
		{http.MethodPost, "/api/domains", `{"x":"y"}`},
		{http.MethodPut, "/api/domains", `{"items":["a"],"attr":{}}`},
		{http.MethodDelete, "/api/domains", `{"items":["a"]}`},
		{http.MethodGet, "/api/dkim/example.com", ""},
		{http.MethodGet, "/api/status/containers", ""},
		{http.MethodPost, "/api/queue/flush", ""},
	}
	for _, u := range upstream {
		if rec := do(h2, u.method, u.path, t2, u.body); rec.Code != http.StatusBadGateway {
			t.Errorf("%s %s: want 502, got %d", u.method, u.path, rec.Code)
		}
	}

	// Logical mailcow failure -> 502 via writeMailcowResults.
	h3 := newAPI(t, mockMailcow(t, 0, `[{"type":"danger","msg":["bad"]}]`).URL, []string{"*"})
	t3 := loginToken(t, h3)
	if rec := do(h3, http.MethodPost, "/api/domains", t3, `{"x":"y"}`); rec.Code != http.StatusBadGateway {
		t.Errorf("want 502 for danger result, got %d", rec.Code)
	}
}

func TestCORS(t *testing.T) {
	// Wildcard origin + preflight.
	h := newAPI(t, mockMailcow(t, 0, "").URL, []string{"*"})
	req := httptest.NewRequest(http.MethodOptions, "/api/domains", nil)
	req.Header.Set("Origin", "https://ui.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight=%d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("wildcard CORS header=%q", got)
	}

	// Specific allowed origin.
	h2 := newAPI(t, mockMailcow(t, 0, "").URL, []string{"https://ui.example"})
	req2 := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req2.Header.Set("Origin", "https://ui.example")
	rec2 := httptest.NewRecorder()
	h2.ServeHTTP(rec2, req2)
	if got := rec2.Header().Get("Access-Control-Allow-Origin"); got != "https://ui.example" {
		t.Errorf("specific CORS header=%q", got)
	}

	// Disallowed origin -> no header.
	req3 := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req3.Header.Set("Origin", "https://evil.example")
	rec3 := httptest.NewRecorder()
	h2.ServeHTTP(rec3, req3)
	if got := rec3.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unexpected CORS header for disallowed origin: %q", got)
	}
}

func TestFrontendServing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>mailfold</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		MailcowBaseURL: "http://mailcow.invalid",
		MailcowAPIKey:  "k",
		AdminUser:      "admin",
		AdminPassword:  "pw",
		SessionTTL:     time.Hour,
		CORSOrigins:    []string{"*"},
		FrontendDir:    dir,
	}
	mc := mailcow.NewClient(cfg.MailcowBaseURL, "k", false)
	authn := auth.New("admin", "pw", time.Hour)
	h := NewServer(cfg, mc, authn, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()

	if rec := do(h, http.MethodGet, "/app.js", "", ""); rec.Code != http.StatusOK {
		t.Errorf("static file app.js=%d", rec.Code)
	}
	rec := do(h, http.MethodGet, "/dashboard", "", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "mailfold") {
		t.Errorf("SPA fallback failed: code=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodGet, "/", "", ""); rec.Code != http.StatusOK {
		t.Errorf("index=%d", rec.Code)
	}
}
