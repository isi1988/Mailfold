package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestDomainAdminCreateCapturesPassword(t *testing.T) {
	h, srv, _ := newAccountTestServer(t, accountTestOpts{withDB: true})
	token := loginToken(t, h)

	rec := do(h, http.MethodPost, "/api/domain-admins", token, `{"username":"da1","password":"secret123","password2":"secret123","domains":"a.com","active":"1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create domain admin = %d: %s", rec.Code, rec.Body.String())
	}
	hash, ok, err := srv.domainAdminStore.GetLoginPassword("da1")
	if err != nil || !ok {
		t.Fatalf("expected a captured login: ok=%v err=%v", ok, err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte("secret123")) != nil {
		t.Error("captured hash does not match the password that was set")
	}
}

func TestDomainAdminCreateWithoutDBStillReachesMailcow(t *testing.T) {
	// No local store configured: the mailcow-side create must still succeed
	// (password capture is a bonus, not a precondition).
	h, _, _ := newAccountTestServer(t, accountTestOpts{})
	token := loginToken(t, h)
	rec := do(h, http.MethodPost, "/api/domain-admins", token, `{"username":"da1","password":"secret123","password2":"secret123","domains":"a.com","active":"1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create domain admin without DB = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDomainAdminEditUpdatesPasswordOnlyWhenProvided(t *testing.T) {
	h, srv, _ := newAccountTestServer(t, accountTestOpts{withDB: true})
	token := loginToken(t, h)

	if rec := do(h, http.MethodPost, "/api/domain-admins", token, `{"username":"da1","password":"first-pw","password2":"first-pw","domains":"a.com","active":"1"}`); rec.Code != http.StatusOK {
		t.Fatalf("create = %d", rec.Code)
	}

	// Editing only the domains (no password) must not disturb the stored hash.
	before, _, _ := srv.domainAdminStore.GetLoginPassword("da1")
	rec := do(h, http.MethodPut, "/api/domain-admins", token, `{"items":["da1"],"attr":{"domains":"a.com,b.com"}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("edit = %d: %s", rec.Code, rec.Body.String())
	}
	after, _, _ := srv.domainAdminStore.GetLoginPassword("da1")
	if before != after {
		t.Error("editing without a password should not change the stored hash")
	}

	// Editing with a new password does change it.
	rec = do(h, http.MethodPut, "/api/domain-admins", token, `{"items":["da1"],"attr":{"password":"second-pw"}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("edit with password = %d: %s", rec.Code, rec.Body.String())
	}
	hash, _, _ := srv.domainAdminStore.GetLoginPassword("da1")
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte("second-pw")) != nil {
		t.Error("expected the stored hash to match the new password")
	}
}

func TestDomainAdminDeleteRemovesLogin(t *testing.T) {
	h, srv, _ := newAccountTestServer(t, accountTestOpts{withDB: true})
	token := loginToken(t, h)

	if rec := do(h, http.MethodPost, "/api/domain-admins", token, `{"username":"da1","password":"pw","password2":"pw","domains":"a.com","active":"1"}`); rec.Code != http.StatusOK {
		t.Fatalf("create = %d", rec.Code)
	}
	if _, ok, _ := srv.domainAdminStore.GetLoginPassword("da1"); !ok {
		t.Fatal("expected a captured login before delete")
	}

	rec := do(h, http.MethodDelete, "/api/domain-admins", token, `{"items":["da1"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete = %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok, _ := srv.domainAdminStore.GetLoginPassword("da1"); ok {
		t.Error("expected the login to be removed after delete")
	}
}

func TestDomainAdminListIsRawPassthrough(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{withDB: true})
	token := loginToken(t, h)
	rec := do(h, http.MethodGet, "/api/domain-admins", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d", rec.Code)
	}
	var body any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("list response is not valid JSON: %v", err)
	}
}
