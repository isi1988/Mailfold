package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/audit"
)

func TestAuditLogRecordsAdminLoginAndLogout(t *testing.T) {
	h, srv, _ := newAccountTestServer(t, accountTestOpts{withDB: true})

	// A failed login is recorded before any session exists.
	if rec := do(h, http.MethodPost, "/api/auth/login", "", `{"user":"admin","password":"wrong"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login = %d", rec.Code)
	}
	token := loginToken(t, h)
	if rec := do(h, http.MethodPost, "/api/auth/logout", token, ""); rec.Code != http.StatusOK {
		t.Fatalf("logout = %d", rec.Code)
	}

	entries, total, err := srv.auditStore.List(50, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3 (login_failed, login, logout), entries=%+v", total, entries)
	}
	// Newest first: logout, login, login_failed.
	assertEntry(t, entries[2], "admin", "admin", "login_failed", http.StatusUnauthorized)
	assertEntry(t, entries[1], "admin", "admin", "login", http.StatusOK)
	assertEntry(t, entries[0], "admin", "admin", "logout", http.StatusOK)
}

func TestAuditLogRecordsMutatingActionsNotReads(t *testing.T) {
	h, srv, _ := newAccountTestServer(t, accountTestOpts{withDB: true, withEncKey: true})
	token := loginToken(t, h)

	// A GET must not be audited...
	if rec := do(h, http.MethodGet, "/api/auth/me", token, ""); rec.Code != http.StatusOK {
		t.Fatalf("me = %d", rec.Code)
	}
	// ...but a mutating POST (regardless of its own business-logic outcome) is.
	if rec := do(h, http.MethodPost, "/api/account/2fa/enroll", token, `{"current_password":"wrong"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("enroll with wrong password = %d", rec.Code)
	}

	entries, _, err := srv.auditStore.List(50, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Only the login itself and the mutating POST should appear — not the GET.
	for _, e := range entries {
		if strings.Contains(e.Action, "GET") {
			t.Errorf("a GET request must not be audited: %+v", e)
		}
	}
	found := false
	for _, e := range entries {
		if e.Action == "POST /api/account/2fa/enroll" && e.ActorType == "admin" && e.Actor == "admin" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a POST /api/account/2fa/enroll entry, got %+v", entries)
	}
}

func TestAuditLogListEndpoint(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{withDB: true})
	token := loginToken(t, h)

	rec := do(h, http.MethodGet, "/api/audit-log", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("audit-log list = %d, body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Entries []audit.Entry `json:"entries"`
		Total   int           `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Total < 1 || len(out.Entries) < 1 {
		t.Fatalf("expected at least the login entry, got total=%d entries=%+v", out.Total, out.Entries)
	}

	// Unauthenticated callers are rejected.
	if rec := do(h, http.MethodGet, "/api/audit-log", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("unauth audit-log = %d, want 401", rec.Code)
	}
}

func TestAuditLogDisabledWithoutDB(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{withDB: false})
	// Without a DB there is no admin account store either, so login itself is
	// unavailable — exercise the list endpoint's nil-store fallback directly
	// via a server that has no session to authenticate with is not meaningful
	// here; instead confirm the store is nil and the handler degrades to an
	// empty page rather than panicking, using the admin's bootstrap password
	// (which works even without a DB-backed account override).
	token := loginToken(t, h)
	rec := do(h, http.MethodGet, "/api/audit-log", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("audit-log without DB = %d, want 200 with an empty page", rec.Code)
	}
	var out struct {
		Entries []audit.Entry `json:"entries"`
		Total   int           `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Total != 0 || len(out.Entries) != 0 {
		t.Errorf("expected an empty page without a DB, got total=%d entries=%+v", out.Total, out.Entries)
	}
}

func TestAuditLogRecordsDomainAdminLoginAndLogout(t *testing.T) {
	mock := &domainAdminMailcowMock{username: "da1", domains: []string{"a.com"}, active: 1}
	h, srv := newDomainAdminTestServer(t, mock)
	if err := srv.domainAdminStore.SetLoginPassword("da1", hashPassword(t, "correct-pw"), time.Now()); err != nil {
		t.Fatalf("SetLoginPassword: %v", err)
	}

	if rec := do(h, http.MethodPost, "/api/auth/domain-admin/login", "", `{"user":"da1","password":"wrong"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login = %d", rec.Code)
	}
	rec := do(h, http.MethodPost, "/api/auth/domain-admin/login", "", `{"user":"da1","password":"correct-pw"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login = %d", rec.Code)
	}
	var res struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if rec := do(h, http.MethodPost, "/api/auth/domain-admin/logout", res.Token, ""); rec.Code != http.StatusOK {
		t.Fatalf("logout = %d", rec.Code)
	}

	entries, total, err := srv.auditStore.List(50, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3, entries=%+v", total, entries)
	}
	assertEntry(t, entries[2], "domain_admin", "da1", "login_failed", http.StatusUnauthorized)
	assertEntry(t, entries[1], "domain_admin", "da1", "login", http.StatusOK)
	assertEntry(t, entries[0], "domain_admin", "da1", "logout", http.StatusOK)
}

func assertEntry(t *testing.T, e audit.Entry, actorType, actor, action string, status int) {
	t.Helper()
	if e.ActorType != actorType || e.Actor != actor || e.Action != action || e.Status != status {
		t.Errorf("entry = %+v, want actor_type=%q actor=%q action=%q status=%d", e, actorType, actor, action, status)
	}
}
