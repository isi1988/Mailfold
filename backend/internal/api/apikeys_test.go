package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-imap/backend/memory"
	"github.com/emersion/go-imap/server"
	"github.com/isi1988/Mailfold/backend/internal/apikey"
	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
)

// appPwMock is a tiny stateful stand-in for mailcow's app-password endpoints so
// the mint/revoke flow can be exercised without a real mailcow.
type appPwMock struct {
	mu       sync.Mutex
	byName   map[string]int
	next     int
	noReturn bool // when true, GET reports no app-passwords (simulates lost id)
}

func (m *appPwMock) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/add/app-passwd", func(w http.ResponseWriter, r *http.Request) {
		var attr map[string]any
		_ = json.NewDecoder(r.Body).Decode(&attr)
		name, _ := attr["app_name"].(string)
		m.mu.Lock()
		m.next++
		m.byName[name] = m.next
		m.mu.Unlock()
		_, _ = io.WriteString(w, `[{"type":"success","msg":"app_passwd_added"}]`)
	})
	mux.HandleFunc("/api/v1/delete/app-passwd", func(w http.ResponseWriter, r *http.Request) {
		var ids []string
		_ = json.NewDecoder(r.Body).Decode(&ids)
		m.mu.Lock()
		for name, id := range m.byName {
			for _, s := range ids {
				if strconv.Itoa(id) == s {
					delete(m.byName, name)
				}
			}
		}
		m.mu.Unlock()
		_, _ = io.WriteString(w, `[{"type":"success","msg":"app_passwd_removed"}]`)
	})
	mux.HandleFunc("/api/v1/get/app-passwd/all/", func(w http.ResponseWriter, r *http.Request) {
		type entry struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		m.mu.Lock()
		out := make([]entry, 0, len(m.byName))
		if !m.noReturn {
			for name, id := range m.byName {
				out = append(out, entry{ID: id, Name: name})
			}
		}
		m.mu.Unlock()
		_ = json.NewEncoder(w).Encode(out)
	})
	return mux
}

func newAPIKeyServer(t *testing.T, rateMax int) (*Server, http.Handler) {
	return newAPIKeyServerMock(t, rateMax, &appPwMock{byName: map[string]int{}})
}

func newAPIKeyServerMock(t *testing.T, rateMax int, mock *appPwMock) (*Server, http.Handler) {
	t.Helper()
	imapSrv := server.New(memory.New())
	imapSrv.AllowInsecureAuth = true
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = imapSrv.Serve(ln) }()
	t.Cleanup(func() { _ = imapSrv.Close() })

	mcSrv := httptest.NewServer(mock.handler())
	t.Cleanup(mcSrv.Close)

	master := make([]byte, 32)
	for i := range master {
		master[i] = byte(i + 1)
	}
	cfg := &config.Config{
		MailcowBaseURL:      mcSrv.URL,
		MailcowAPIKey:       "k",
		AdminUser:           "admin",
		AdminPassword:       "pw",
		SessionTTL:          time.Hour,
		CORSOrigins:         []string{"*"},
		IMAPAddr:            ln.Addr().String(),
		MailUseTLS:          false,
		WebmailSessionTTL:   time.Hour,
		DBPath:              t.TempDir() + "/db.sqlite",
		APIKeyEnabled:       true,
		APIKeyMasterKey:     master,
		APIKeyRateMax:       rateMax,
		APIKeyRateWindow:    time.Minute,
		APIKeyMaxRecipients: 5,
	}
	mc := mailcow.NewClient(cfg.MailcowBaseURL, cfg.MailcowAPIKey, false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, time.Hour)
	srv := NewServer(cfg, mc, authn, ratelimit.New(0, time.Minute), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return srv, srv.Handler()
}

func adminLogin(t *testing.T, h http.Handler) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"user":"admin","password":"pw"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin login failed: %d", rec.Code)
	}
	var out struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Token == "" {
		t.Fatal("empty admin token")
	}
	return out.Token
}

func doKey(t *testing.T, h http.Handler, method, path, bearer, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestAPIKeyMintListRevoke(t *testing.T) {
	_, h := newAPIKeyServer(t, 120)
	admin := adminLogin(t, h)

	// Mint a read-only key.
	rec := doKey(t, h, http.MethodPost, "/api/apikeys", admin, `{"mailbox":"davtest@cortexus.ru","label":"bot","scopes":["mail:read"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("mint = %d, body=%s", rec.Code, rec.Body.String())
	}
	var minted struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &minted)
	if !strings.HasPrefix(minted.Token, apikey.TokenPrefix) || minted.ID == "" {
		t.Fatalf("bad mint response: %s", rec.Body.String())
	}
	readTok := minted.Token

	// List must not leak the token/secret.
	rec = doKey(t, h, http.MethodGet, "/api/apikeys", admin, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), minted.ID) {
		t.Fatalf("list = %d, body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), readTok) || strings.Contains(rec.Body.String(), "secret") {
		t.Error("list leaked the token or secret material")
	}

	// Unauthenticated and malformed keys are a uniform 401.
	if rec := doKey(t, h, http.MethodGet, "/api/v1/mail/folders", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no-auth folders = %d, want 401", rec.Code)
	}
	if rec := doKey(t, h, http.MethodGet, "/api/v1/mail/folders", "nonsense", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("malformed token = %d, want 401", rec.Code)
	}

	// A read-only key may not send (scope 403) but may hit a read route.
	if rec := doKey(t, h, http.MethodPost, "/api/v1/mail/send", readTok, `{"to":["x@y.z"],"subject":"hi"}`); rec.Code != http.StatusForbidden {
		t.Errorf("read key send = %d, want 403", rec.Code)
	}
	if rec := doKey(t, h, http.MethodGet, "/api/v1/mail/message?uid=notanumber", readTok, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bad uid = %d, want 400", rec.Code)
	}

	// Revoke, then the key is rejected.
	if rec := doKey(t, h, http.MethodDelete, "/api/apikeys/"+minted.ID, admin, ""); rec.Code != http.StatusOK {
		t.Fatalf("revoke = %d", rec.Code)
	}
	if rec := doKey(t, h, http.MethodGet, "/api/v1/mail/folders", readTok, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked key = %d, want 401", rec.Code)
	}
	// Revoking an unknown id is 404.
	if rec := doKey(t, h, http.MethodDelete, "/api/apikeys/ghost", admin, ""); rec.Code != http.StatusNotFound {
		t.Errorf("revoke ghost = %d, want 404", rec.Code)
	}
}

func TestAPIKeyMintValidation(t *testing.T) {
	_, h := newAPIKeyServer(t, 120)
	admin := adminLogin(t, h)
	if rec := doKey(t, h, http.MethodPost, "/api/apikeys", admin, `{"mailbox":"not a mailbox"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad mailbox = %d, want 400", rec.Code)
	}
	if rec := doKey(t, h, http.MethodPost, "/api/apikeys", admin, `{"mailbox":"a@b.co","scopes":["mail:admin"]}`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad scope = %d, want 400", rec.Code)
	}
	// Management routes require the admin session.
	if rec := doKey(t, h, http.MethodGet, "/api/apikeys", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("unauth list = %d, want 401", rec.Code)
	}
}

// insertKey stores a key directly with a known app-password so the collect/send
// handlers can be exercised against the in-memory IMAP server.
func insertKey(t *testing.T, srv *Server, mailbox, appPw string, scopes []string) string {
	t.Helper()
	token, kid, sha, prefix, err := apikey.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	enc, nonce, err := srv.apikeyCipher.Seal([]byte(appPw))
	if err != nil {
		t.Fatal(err)
	}
	err = srv.apikeyStore.Create(apikey.Record{
		ID: kid, TokenSHA256: sha, Prefix: prefix, Mailbox: mailbox,
		Scopes: apikey.JoinScopes(scopes), SecretEnc: enc, SecretNonce: nonce,
		Created: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestAPIKeyMailAndCaps(t *testing.T) {
	srv, h := newAPIKeyServer(t, 120)
	srv.GCAPIKeys() // exercises the limiter sweep (no-op on empty maps)
	// The in-memory IMAP backend authenticates username/password.
	tok := insertKey(t, srv, "username", "password", apikey.DefaultScopes())

	if rec := doKey(t, h, http.MethodGet, "/api/v1/mail/folders", tok, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "INBOX") {
		t.Fatalf("folders = %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec := doKey(t, h, http.MethodGet, "/api/v1/mail/messages?limit=10", tok, ""); rec.Code != http.StatusOK {
		t.Errorf("messages = %d", rec.Code)
	}
	if rec := doKey(t, h, http.MethodGet, "/api/v1/mail/search?q=test", tok, ""); rec.Code != http.StatusOK {
		t.Errorf("search = %d", rec.Code)
	}

	// The remaining read/write verbs must at least route to their handler (the
	// message may or may not exist in the demo mailbox, so any non-auth status is
	// acceptable — we assert they are neither 401 nor 403).
	for _, c := range []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/v1/mail/message?uid=1", ""},
		{http.MethodGet, "/api/v1/mail/attachment?uid=1&index=0", ""},
		{http.MethodPost, "/api/v1/mail/flag", `{"folder":"INBOX","uid":1,"set":true}`},
		{http.MethodDelete, "/api/v1/mail/message", `{"folder":"INBOX","uid":1}`},
	} {
		rec := doKey(t, h, c.method, c.path, tok, c.body)
		if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
			t.Errorf("%s %s auth-gated unexpectedly: %d", c.method, c.path, rec.Code)
		}
	}

	// Recipient cap (max 5).
	tooMany := `{"to":["1@x.z","2@x.z","3@x.z","4@x.z","5@x.z","6@x.z"],"subject":"hi","text":"y"}`
	if rec := doKey(t, h, http.MethodPost, "/api/v1/mail/send", tok, tooMany); rec.Code != http.StatusBadRequest {
		t.Errorf("too many recipients = %d, want 400", rec.Code)
	}
	// Body-size cap.
	big := `{"to":["1@x.z"],"subject":"hi","text":"` + strings.Repeat("a", (1<<20)+10) + `"}`
	if rec := doKey(t, h, http.MethodPost, "/api/v1/mail/send", tok, big); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body = %d, want 413", rec.Code)
	}
}

func TestAPIKeyMintIDRecoveryFails(t *testing.T) {
	// The upstream app-password is created but cannot be uniquely identified
	// afterwards, so the mint must fail (502) rather than persist an
	// unrevokable key, and the compensating delete must run.
	_, h := newAPIKeyServerMock(t, 120, &appPwMock{byName: map[string]int{}, noReturn: true})
	admin := adminLogin(t, h)
	rec := doKey(t, h, http.MethodPost, "/api/apikeys", admin, `{"mailbox":"a@b.co"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("mint with lost id = %d, want 502", rec.Code)
	}
	// No key should have been persisted.
	list := doKey(t, h, http.MethodGet, "/api/apikeys", admin, "")
	if !strings.Contains(list.Body.String(), "[]") {
		t.Errorf("no key should be stored, got %s", list.Body.String())
	}
}

func TestAPIKeyRateLimit(t *testing.T) {
	srv, h := newAPIKeyServer(t, 1) // per-key budget of 1/window
	tok := insertKey(t, srv, "username", "password", apikey.DefaultScopes())
	if rec := doKey(t, h, http.MethodGet, "/api/v1/mail/folders", tok, ""); rec.Code != http.StatusOK {
		t.Fatalf("first call = %d, want 200", rec.Code)
	}
	if rec := doKey(t, h, http.MethodGet, "/api/v1/mail/folders", tok, ""); rec.Code != http.StatusTooManyRequests {
		t.Errorf("second call = %d, want 429", rec.Code)
	}
}
