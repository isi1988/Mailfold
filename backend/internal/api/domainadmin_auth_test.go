package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
)

// hashPassword bcrypt-hashes password for tests that seed a domain admin's
// login directly through the store, bypassing the create/edit HTTP flow
// (which is exercised separately in domainadmins_test.go).
func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword: %v", err)
	}
	return string(hash)
}

// domainAdminMailcowMock is a stateful stand-in for mailcow's domain-admin
// list endpoint, so login can confirm current scope/active status.
type domainAdminMailcowMock struct {
	username string
	domains  []string
	active   int
}

func (m *domainAdminMailcowMock) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/get/domain-admin/all", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"username":         m.username,
			"active":           m.active,
			"selected_domains": m.domains,
		}})
	})
	return mux
}

func newDomainAdminTestServer(t *testing.T, mock *domainAdminMailcowMock) (http.Handler, *Server) {
	t.Helper()
	mcSrv := httptest.NewServer(mock.handler())
	t.Cleanup(mcSrv.Close)

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 5)
	}
	cfg := &config.Config{
		MailcowBaseURL:  mcSrv.URL,
		MailcowAPIKey:   "k",
		AdminUser:       "admin",
		AdminPassword:   "pw",
		SessionTTL:      time.Hour,
		CORSOrigins:     []string{"*"},
		LoginRateMax:    1000,
		LoginRateWindow: time.Minute,
		DBDriver:        "sqlite",
		DBPath:          t.TempDir() + "/domainadmin.db",
		AdminEncKey:     key,
	}
	mc := mailcow.NewClient(cfg.MailcowBaseURL, cfg.MailcowAPIKey, false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	limiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	srv := NewServer(cfg, mc, authn, limiter, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return srv.Handler(), srv
}

func TestDomainAdminLoginDisabledWithoutDB(t *testing.T) {
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

	rec := do(h, http.MethodPost, "/api/auth/domain-admin/login", "", `{"user":"da1","password":"pw"}`)
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("login without DB = %d, want 501", rec.Code)
	}
}

func TestDomainAdminLoginSuccessAndMe(t *testing.T) {
	mock := &domainAdminMailcowMock{username: "da1", domains: []string{"a.com", "b.com"}, active: 1}
	h, srv := newDomainAdminTestServer(t, mock)

	if err := srv.domainAdminStore.SetLoginPassword("da1", hashPassword(t, "correct-pw"), time.Now()); err != nil {
		t.Fatalf("SetLoginPassword: %v", err)
	}

	// Wrong password.
	if rec := do(h, http.MethodPost, "/api/auth/domain-admin/login", "", `{"user":"da1","password":"wrong"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("login wrong password = %d, want 401", rec.Code)
	}
	// Unknown user.
	if rec := do(h, http.MethodPost, "/api/auth/domain-admin/login", "", `{"user":"nobody","password":"x"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("login unknown user = %d, want 401", rec.Code)
	}

	rec := do(h, http.MethodPost, "/api/auth/domain-admin/login", "", `{"user":"da1","password":"correct-pw"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login = %d: %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Token   string   `json:"token"`
		User    string   `json:"user"`
		Domains []string `json:"domains"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Token == "" || res.User != "da1" || len(res.Domains) != 2 {
		t.Fatalf("unexpected login response: %+v", res)
	}

	// The session actually authenticates and reports the current scope.
	meRec := do(h, http.MethodGet, "/api/auth/domain-admin/me", res.Token, "")
	if meRec.Code != http.StatusOK {
		t.Fatalf("me = %d", meRec.Code)
	}
	var me struct {
		User    string   `json:"user"`
		Domains []string `json:"domains"`
	}
	_ = json.Unmarshal(meRec.Body.Bytes(), &me)
	if me.User != "da1" || len(me.Domains) != 2 {
		t.Errorf("unexpected me response: %+v", me)
	}

	// Logout invalidates the session.
	if rec := do(h, http.MethodPost, "/api/auth/domain-admin/logout", res.Token, ""); rec.Code != http.StatusOK {
		t.Errorf("logout = %d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/auth/domain-admin/me", res.Token, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("me after logout = %d, want 401", rec.Code)
	}
}

func TestDomainAdminLoginRejectsDeactivated(t *testing.T) {
	mock := &domainAdminMailcowMock{username: "da1", domains: []string{"a.com"}, active: 0}
	h, srv := newDomainAdminTestServer(t, mock)
	if err := srv.domainAdminStore.SetLoginPassword("da1", hashPassword(t, "pw"), time.Now()); err != nil {
		t.Fatalf("SetLoginPassword: %v", err)
	}
	rec := do(h, http.MethodPost, "/api/auth/domain-admin/login", "", `{"user":"da1","password":"pw"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("login for a deactivated domain admin = %d, want 401", rec.Code)
	}
}

func TestRequireDomainAdminRejectsMissingToken(t *testing.T) {
	h, _ := newDomainAdminTestServer(t, &domainAdminMailcowMock{})
	if rec := do(h, http.MethodGet, "/api/auth/domain-admin/me", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("me without token = %d, want 401", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/auth/domain-admin/me", "not-a-real-token", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("me with bogus token = %d, want 401", rec.Code)
	}
}
