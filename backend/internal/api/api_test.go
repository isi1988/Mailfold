package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		MailcowBaseURL:  mcURL,
		MailcowAPIKey:   "k",
		AdminUser:       "admin",
		AdminPassword:   "pw",
		SessionTTL:      time.Hour,
		CORSOrigins:     origins,
		LoginRateMax:    100, // high enough not to interfere with normal tests
		LoginRateWindow: time.Minute,
		MaxBodyBytes:    1 << 20,
	}
	mc := mailcow.NewClient(mcURL, "k", false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	limiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(cfg, mc, authn, limiter, logger).Handler()
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
		{http.MethodPost, "/api/queue/delete-all", "", 200},
		{http.MethodGet, "/api/logs/postfix", "", 200},
		{http.MethodGet, "/api/logs/postfix?count=5", "", 200},
		{http.MethodGet, "/api/logs/postfix?count=0", "", 200},
		{http.MethodGet, "/api/logs/postfix?count=99999", "", 200},
		{http.MethodGet, "/api/logs/unknownsvc", "", 400},
		{http.MethodGet, "/api/fail2ban", "", 200},
		{http.MethodPut, "/api/fail2ban", attr, 200},
		{http.MethodGet, "/api/quarantine", "", 200},
		{http.MethodPut, "/api/quarantine", edit, 200},
		{http.MethodDelete, "/api/quarantine", del, 200},
		{http.MethodGet, "/api/policy/allow/example.com", "", 200},
		{http.MethodGet, "/api/policy/deny/example.com", "", 200},
		{http.MethodPost, "/api/policy", attr, 200},
		{http.MethodDelete, "/api/policy", del, 200},
		{http.MethodGet, "/api/status/containers", "", 200},
		{http.MethodGet, "/api/status/version", "", 200},
		{http.MethodGet, "/api/status/vmail", "", 200},
		// Phase A resources.
		{http.MethodGet, "/api/domain-admins", "", 200},
		{http.MethodPost, "/api/domain-admins", attr, 200},
		{http.MethodPut, "/api/domain-admins", edit, 200},
		{http.MethodDelete, "/api/domain-admins", del, 200},
		{http.MethodGet, "/api/resources", "", 200},
		{http.MethodPost, "/api/resources", attr, 200},
		{http.MethodPut, "/api/resources", edit, 200},
		{http.MethodDelete, "/api/resources", del, 200},
		{http.MethodGet, "/api/app-passwords/user@example.com", "", 200},
		{http.MethodPost, "/api/app-passwords", attr, 200},
		{http.MethodDelete, "/api/app-passwords", del, 200},
		{http.MethodGet, "/api/oauth2-clients", "", 200},
		{http.MethodPost, "/api/oauth2-clients", attr, 200},
		{http.MethodDelete, "/api/oauth2-clients", del, 200},
		{http.MethodGet, "/api/forwarding-hosts", "", 200},
		{http.MethodPost, "/api/forwarding-hosts", attr, 200},
		{http.MethodDelete, "/api/forwarding-hosts", del, 200},
		{http.MethodGet, "/api/transports", "", 200},
		{http.MethodPost, "/api/transports", attr, 200},
		{http.MethodPut, "/api/transports", edit, 200},
		{http.MethodDelete, "/api/transports", del, 200},
		{http.MethodGet, "/api/relayhosts", "", 200},
		{http.MethodPost, "/api/relayhosts", attr, 200},
		{http.MethodPut, "/api/relayhosts", edit, 200},
		{http.MethodDelete, "/api/relayhosts", del, 200},
		{http.MethodGet, "/api/tls-policies", "", 200},
		{http.MethodPost, "/api/tls-policies", attr, 200},
		{http.MethodDelete, "/api/tls-policies", del, 200},
		{http.MethodGet, "/api/bcc", "", 200},
		{http.MethodPost, "/api/bcc", attr, 200},
		{http.MethodDelete, "/api/bcc", del, 200},
		{http.MethodGet, "/api/recipient-maps", "", 200},
		{http.MethodPost, "/api/recipient-maps", attr, 200},
		{http.MethodDelete, "/api/recipient-maps", del, 200},
		// Phase A2 resources.
		{http.MethodGet, "/api/admins", "", 200},
		{http.MethodPost, "/api/admins", attr, 200},
		{http.MethodPut, "/api/admins", edit, 200},
		{http.MethodDelete, "/api/admins", del, 200},
		{http.MethodGet, "/api/domain-templates", "", 200},
		{http.MethodPost, "/api/domain-templates", attr, 200},
		{http.MethodPut, "/api/domain-templates", edit, 200},
		{http.MethodDelete, "/api/domain-templates", del, 200},
		{http.MethodGet, "/api/mailbox-templates", "", 200},
		{http.MethodPost, "/api/mailbox-templates", attr, 200},
		{http.MethodPut, "/api/mailbox-templates", edit, 200},
		{http.MethodDelete, "/api/mailbox-templates", del, 200},
		{http.MethodGet, "/api/rspamd-settings", "", 200},
		{http.MethodPost, "/api/rspamd-settings", attr, 200},
		{http.MethodDelete, "/api/rspamd-settings", del, 200},
		{http.MethodGet, "/api/ratelimits/mailbox", "", 200},
		{http.MethodPut, "/api/ratelimits/mailbox", edit, 200},
		{http.MethodGet, "/api/ratelimits/domain", "", 200},
		{http.MethodPut, "/api/ratelimits/domain", edit, 200},
		{http.MethodPut, "/api/pushover", edit, 200},
		// Phase W2 resources.
		{http.MethodGet, "/api/filters", "", 200},
		{http.MethodPost, "/api/filters", attr, 200},
		{http.MethodPut, "/api/filters", edit, 200},
		{http.MethodDelete, "/api/filters", del, 200},
		{http.MethodGet, "/api/temp-aliases/user@example.com", "", 200},
		{http.MethodPost, "/api/temp-aliases", attr, 200},
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
		{http.MethodPost, "/api/queue/delete-all", ""},
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
	limiter := ratelimit.New(0, time.Minute)
	h := NewServer(cfg, mc, authn, limiter, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()

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

func TestSecurityHeadersAndRequestID(t *testing.T) {
	h := newAPI(t, mockMailcow(t, 0, "").URL, []string{"*"})

	rec := do(h, http.MethodGet, "/api/health", "", "")
	for k, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("header %s = %q, want %q", k, got, want)
		}
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Error("expected a generated X-Request-Id header")
	}

	// A client-supplied request id must be echoed back unchanged.
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("X-Request-Id", "trace-123")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if got := rec2.Header().Get("X-Request-Id"); got != "trace-123" {
		t.Errorf("X-Request-Id = %q, want the supplied trace-123", got)
	}
}

func TestReadiness(t *testing.T) {
	// mailcow reachable -> ready.
	h := newAPI(t, mockMailcow(t, 0, "").URL, []string{"*"})
	if rec := do(h, http.MethodGet, "/api/health/ready", "", ""); rec.Code != http.StatusOK {
		t.Errorf("ready with healthy mailcow = %d, want 200", rec.Code)
	}
	// mailcow failing -> not ready.
	h2 := newAPI(t, mockMailcow(t, http.StatusInternalServerError, "").URL, []string{"*"})
	if rec := do(h2, http.MethodGet, "/api/health/ready", "", ""); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("ready with failing mailcow = %d, want 503", rec.Code)
	}
}

func TestLoginRateLimit(t *testing.T) {
	// Build a server with a tiny login budget of two attempts per window.
	cfg := &config.Config{
		MailcowBaseURL: mockMailcow(t, 0, "").URL,
		MailcowAPIKey:  "k",
		AdminUser:      "admin",
		AdminPassword:  "pw",
		SessionTTL:     time.Hour,
		CORSOrigins:    []string{"*"},
	}
	mc := mailcow.NewClient(cfg.MailcowBaseURL, "k", false)
	authn := auth.New("admin", "pw", time.Hour)
	limiter := ratelimit.New(2, time.Minute)
	h := NewServer(cfg, mc, authn, limiter, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()

	body := `{"user":"admin","password":"wrong"}`
	// The first two attempts are allowed (and fail auth with 401).
	for i := 1; i <= 2; i++ {
		if rec := do(h, http.MethodPost, "/api/auth/login", "", body); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, want 401", i, rec.Code)
		}
	}
	// The third attempt is throttled with 429 + Retry-After.
	rec := do(h, http.MethodPost, "/api/auth/login", "", body)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt 3 = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected a Retry-After header on the 429 response")
	}
}

func TestDocsEndpoints(t *testing.T) {
	h := newAPI(t, mockMailcow(t, 0, "").URL, []string{"*"})

	rec := do(h, http.MethodGet, "/api/openapi.yaml", "", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "openapi:") {
		t.Errorf("openapi spec: code=%d body-start=%.20q", rec.Code, rec.Body.String())
	}
	rec2 := do(h, http.MethodGet, "/api/docs", "", "")
	if rec2.Code != http.StatusOK || !strings.Contains(rec2.Body.String(), "swagger-ui") {
		t.Errorf("docs page: code=%d", rec2.Code)
	}
}

func TestWebmailFlow(t *testing.T) {
	// In-memory IMAP server (user "username"/"password", sample INBOX).
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
		LoginRateMax:      100,
		LoginRateWindow:   time.Minute,
		IMAPAddr:          ln.Addr().String(),
		MailUseTLS:        false,
		WebmailSessionTTL: time.Hour,
	}
	mc := mailcow.NewClient(cfg.MailcowBaseURL, "k", false)
	authn := auth.New("admin", "pw", time.Hour)
	limiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	h := NewServer(cfg, mc, authn, limiter, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()

	if rec := do(h, http.MethodGet, "/api/webmail/folders", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("folders without token = %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/webmail/login", "", `{"email":"username","password":"wrong"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad webmail login = %d", rec.Code)
	}

	rec := do(h, http.MethodPost, "/api/webmail/login", "", `{"email":"username","password":"password"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("webmail login = %d", rec.Code)
	}
	var out struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Token == "" {
		t.Fatal("webmail login returned no token")
	}
	tok := out.Token

	if rec := do(h, http.MethodGet, "/api/webmail/folders", tok, ""); rec.Code != http.StatusOK {
		t.Errorf("folders = %d", rec.Code)
	}

	rec = do(h, http.MethodGet, "/api/webmail/messages?folder=INBOX", tok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("messages = %d", rec.Code)
	}
	var msgs []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &msgs)
	if len(msgs) > 0 {
		uid := int(msgs[0]["uid"].(float64))
		if r2 := do(h, http.MethodGet, fmt.Sprintf("/api/webmail/message?folder=INBOX&uid=%d", uid), tok, ""); r2.Code != http.StatusOK {
			t.Errorf("read message = %d", r2.Code)
		}
	}
	if rec := do(h, http.MethodGet, "/api/webmail/message?folder=INBOX&uid=notanumber", tok, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("invalid uid = %d", rec.Code)
	}

	// Search and attachments.
	if rec := do(h, http.MethodGet, "/api/webmail/search?folder=INBOX&q=hello", tok, ""); rec.Code != http.StatusOK {
		t.Errorf("search = %d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/webmail/search?folder=INBOX", tok, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("search without q = %d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/webmail/attachment?uid=notanumber", tok, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("attachment with invalid uid = %d", rec.Code)
	}

	// Message actions.
	if len(msgs) > 0 {
		uid := int(msgs[0]["uid"].(float64))
		flagBody := fmt.Sprintf(`{"folder":"INBOX","uid":%d,"flag":"flagged","set":true}`, uid)
		if r2 := do(h, http.MethodPost, "/api/webmail/flag", tok, flagBody); r2.Code != http.StatusOK {
			t.Errorf("flag = %d", r2.Code)
		}
	}
	if r2 := do(h, http.MethodPost, "/api/webmail/flag", tok, "{bad"); r2.Code != http.StatusBadRequest {
		t.Errorf("malformed flag = %d", r2.Code)
	}
	if r2 := do(h, http.MethodPost, "/api/webmail/folders", tok, `{"name":"TestBox"}`); r2.Code != http.StatusOK {
		t.Errorf("create folder = %d", r2.Code)
	}
	if r2 := do(h, http.MethodPost, "/api/webmail/folders", tok, `{"name":""}`); r2.Code != http.StatusBadRequest {
		t.Errorf("create folder with empty name = %d", r2.Code)
	}
	if len(msgs) > 0 {
		uid := int(msgs[0]["uid"].(float64))
		moveBody := fmt.Sprintf(`{"folder":"INBOX","uid":%d,"target":"TestBox"}`, uid)
		if r2 := do(h, http.MethodPost, "/api/webmail/move", tok, moveBody); r2.Code != http.StatusOK {
			t.Errorf("move = %d", r2.Code)
		}
	}
	if r2 := do(h, http.MethodPost, "/api/webmail/delete", tok, "{bad"); r2.Code != http.StatusBadRequest {
		t.Errorf("malformed delete = %d", r2.Code)
	}

	// Send with no SMTP address configured surfaces an upstream error.
	if rec := do(h, http.MethodPost, "/api/webmail/send", tok, `{"to":["x@y.z"],"subject":"s","text":"t"}`); rec.Code != http.StatusBadGateway {
		t.Errorf("send without SMTP = %d, want 502", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/webmail/send", tok, "{bad"); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed send = %d, want 400", rec.Code)
	}

	if rec := do(h, http.MethodPost, "/api/webmail/logout", tok, ""); rec.Code != http.StatusOK {
		t.Errorf("logout = %d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/webmail/folders", tok, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("folders after logout = %d", rec.Code)
	}
}

func TestWebmailNotConfigured(t *testing.T) {
	h := newAPI(t, mockMailcow(t, 0, "").URL, []string{"*"}) // no IMAP address configured
	if rec := do(h, http.MethodPost, "/api/webmail/login", "", `{"email":"u","password":"p"}`); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("webmail login without config = %d, want 503", rec.Code)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	h := newAPI(t, mockMailcow(t, 0, "").URL, []string{"*"})
	// Generate some traffic so the counters are non-empty.
	do(h, http.MethodGet, "/api/health", "", "")
	do(h, http.MethodGet, "/api/domains", "", "") // 401 without a token

	rec := do(h, http.MethodGet, "/metrics", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "mailfold_http_requests_total") {
		t.Errorf("metrics missing request counter:\n%s", body)
	}
	if !strings.Contains(body, "mailfold_http_request_duration_seconds_count") {
		t.Error("metrics missing histogram count")
	}
}
