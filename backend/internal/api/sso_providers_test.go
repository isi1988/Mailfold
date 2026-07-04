package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestSSOProvidersRequireStore(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{}) // no DB, no enc key
	token := loginToken(t, h)
	if rec := do(h, http.MethodGet, "/api/sso-providers", token, ""); rec.Code != http.StatusNotImplemented {
		t.Errorf("list without store = %d, want 501", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/sso-providers", token, `{}`); rec.Code != http.StatusNotImplemented {
		t.Errorf("create without store = %d, want 501", rec.Code)
	}
}

func TestSSOProvidersCreateListEditDelete(t *testing.T) {
	h, srv, _ := newAccountTestServer(t, accountTestOpts{withDB: true, withEncKey: true})
	token := loginToken(t, h)

	createBody := `{"name":"Corp SSO","issuer":"https://idp.example.com","client_id":"cid","client_secret":"csecret","redirect_url":"https://mf.example.com/api/auth/sso/callback","all_domains":true}`
	rec := do(h, http.MethodPost, "/api/sso-providers", token, createBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("create = %d: %s", rec.Code, rec.Body.String())
	}
	var created ssoProviderView
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.ID == 0 || created.Name != "Corp SSO" || !created.Configured || !created.AllDomains {
		t.Fatalf("unexpected create response: %+v", created)
	}
	// The secret itself must never be echoed back.
	if strings.Contains(rec.Body.String(), "csecret") {
		t.Error("client secret must not appear in the response")
	}

	// Missing required fields is rejected.
	if rec := do(h, http.MethodPost, "/api/sso-providers", token, `{"name":"x"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("create with missing fields = %d, want 400", rec.Code)
	}

	// List includes it.
	rec = do(h, http.MethodGet, "/api/sso-providers", token, "")
	var list []ssoProviderView
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("unexpected list: %+v", list)
	}

	// Edit without a new secret keeps the old one configured; verify the
	// underlying store's ciphertext didn't change to confirm "keep current".
	before, _, _ := srv.domainAdminStore.GetProvider(created.ID)
	editBody := `{"id":` + itoa(created.ID) + `,"name":"Renamed","issuer":"https://idp.example.com","client_id":"cid","redirect_url":"https://mf.example.com/api/auth/sso/callback","all_domains":false,"domains":["a.com"],"active":true}`
	rec = do(h, http.MethodPut, "/api/sso-providers", token, editBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("edit = %d: %s", rec.Code, rec.Body.String())
	}
	after, _, _ := srv.domainAdminStore.GetProvider(created.ID)
	if string(after.ClientSecretEnc) != string(before.ClientSecretEnc) {
		t.Error("editing without a new secret should keep the existing ciphertext")
	}
	if after.Name != "Renamed" || after.AllDomains || len(after.Domains) != 1 {
		t.Errorf("edit did not persist: %+v", after)
	}

	// Delete removes it.
	rec = do(h, http.MethodDelete, "/api/sso-providers", token, `{"id":"`+itoa(created.ID)+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete = %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok, _ := srv.domainAdminStore.GetProvider(created.ID); ok {
		t.Error("provider should be gone after delete")
	}
}
