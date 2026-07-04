package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/domainadmin"
)

// newDomainAdminSSOTestServer builds a server with a DB+enc key and a
// logged-in domain-admin session scoped to domains, returning the handler,
// server, and that session's bearer token.
func newDomainAdminSSOTestServer(t *testing.T, username string, domains []string) (http.Handler, *Server, string) {
	t.Helper()
	mock := &domainAdminMailcowMock{username: username, domains: domains, active: 1}
	h, srv := newDomainAdminTestServer(t, mock)
	if err := srv.domainAdminStore.SetLoginPassword(username, hashPassword(t, "pw"), time.Now()); err != nil {
		t.Fatalf("SetLoginPassword: %v", err)
	}
	rec := do(h, http.MethodPost, "/api/auth/domain-admin/login", "", `{"user":"`+username+`","password":"pw"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("domain admin login = %d: %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	return h, srv, res.Token
}

func TestDomainAdminSSOSeesSharedAndOwnProviders(t *testing.T) {
	h, srv, token := newDomainAdminSSOTestServer(t, "da1", []string{"a.com"})

	// A super-admin-created AllDomains provider ...
	encG, nonceG, _ := srv.adminCipher.Seal([]byte("secret"))
	_, err := srv.domainAdminStore.CreateProvider(domainadmin.Provider{
		Name: "Global", Issuer: "https://idp1.example.com", ClientID: "c1",
		ClientSecretEnc: encG, ClientSecretNonce: nonceG, AllDomains: true, Active: true,
	}, time.Now())
	if err != nil {
		t.Fatalf("CreateProvider (global): %v", err)
	}
	// ... one scoped to a domain this admin does NOT manage ...
	encO, nonceO, _ := srv.adminCipher.Seal([]byte("secret"))
	_, err = srv.domainAdminStore.CreateProvider(domainadmin.Provider{
		Name: "OtherDomainOnly", Domains: []string{"other.com"},
		ClientSecretEnc: encO, ClientSecretNonce: nonceO, Active: true,
	}, time.Now())
	if err != nil {
		t.Fatalf("CreateProvider (other): %v", err)
	}

	rec := do(h, http.MethodGet, "/api/domain-admin/sso-providers", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", rec.Code, rec.Body.String())
	}
	var list []domainAdminProviderView
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0].Name != "Global" {
		t.Fatalf("expected to see only the Global provider, got %+v", list)
	}
	if list[0].Editable {
		t.Error("a super-admin's shared provider must not be editable by a domain admin")
	}
}

func TestDomainAdminSSOCreateScopedToOwnDomains(t *testing.T) {
	h, srv, token := newDomainAdminSSOTestServer(t, "da1", []string{"a.com", "b.com"})

	// Attempting to scope to a domain outside their own management is rejected.
	badBody := `{"name":"Mine","issuer":"https://idp.example.com","client_id":"c","client_secret":"s","redirect_url":"https://mf.example.com/cb","domains":["other.com"]}`
	if rec := do(h, http.MethodPost, "/api/domain-admin/sso-providers", token, badBody); rec.Code != http.StatusForbidden {
		t.Errorf("create scoped to a domain outside management = %d, want 403: %s", rec.Code, rec.Body.String())
	}

	// A provider scoped to (a subset of) their own domains succeeds and is
	// forced to created_by=them, all_domains=false regardless of what's asked.
	goodBody := `{"name":"Mine","issuer":"https://idp.example.com","client_id":"c","client_secret":"s","redirect_url":"https://mf.example.com/cb","domains":["a.com"],"all_domains":true}`
	rec := do(h, http.MethodPost, "/api/domain-admin/sso-providers", token, goodBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("create = %d: %s", rec.Code, rec.Body.String())
	}
	var created domainAdminProviderView
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.AllDomains {
		t.Error("a domain admin must never be able to create an all-domains provider")
	}
	if !created.Editable {
		t.Error("a domain admin should be able to edit their own provider")
	}
	stored, _, _ := srv.domainAdminStore.GetProvider(created.ID)
	if stored.CreatedBy != "da1" {
		t.Errorf("CreatedBy = %q, want da1", stored.CreatedBy)
	}

	// A second domain admin (managing a different domain, scoped away from
	// "a.com"/"b.com") cannot edit or delete the first admin's provider. Its
	// session is seeded directly against the same server/store — mailcow's
	// domain-admin list is a single fake endpoint per test server, so a
	// second identity is easiest to introduce this way rather than a second
	// full server.
	da2Token, _, err := srv.domainAdminSessions.Create("da2", []string{"c.com"})
	if err != nil {
		t.Fatalf("seed da2 session: %v", err)
	}
	editBody := `{"id":` + itoa(created.ID) + `,"name":"hijacked","issuer":"x","client_id":"c","redirect_url":"https://mf.example.com/cb"}`
	if rec := do(h, http.MethodPut, "/api/domain-admin/sso-providers", da2Token, editBody); rec.Code != http.StatusNotFound {
		t.Errorf("edit someone else's provider = %d, want 404", rec.Code)
	}
	deleteBody := `{"id":"` + itoa(created.ID) + `"}`
	if rec := do(h, http.MethodDelete, "/api/domain-admin/sso-providers", da2Token, deleteBody); rec.Code != http.StatusNotFound {
		t.Errorf("delete someone else's provider = %d, want 404", rec.Code)
	}

	// The owner can edit and delete it.
	ownEdit := `{"id":` + itoa(created.ID) + `,"name":"Renamed","issuer":"https://idp.example.com","client_id":"c","redirect_url":"https://mf.example.com/cb","domains":["b.com"]}`
	if rec := do(h, http.MethodPut, "/api/domain-admin/sso-providers", token, ownEdit); rec.Code != http.StatusOK {
		t.Fatalf("owner edit = %d: %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodDelete, "/api/domain-admin/sso-providers", token, deleteBody); rec.Code != http.StatusOK {
		t.Fatalf("owner delete = %d: %s", rec.Code, rec.Body.String())
	}
}
