package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

func TestListSharedMailboxesIncludesMembers(t *testing.T) {
	h, _, _ := newSharedMailboxTestServer(t)
	adminToken := loginToken(t, h)
	createSharedMailbox(t, h, adminToken)

	rec := doJSON(t, h, http.MethodGet, "/api/shared-mailboxes", adminToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list shared mailboxes = %d: %s", rec.Code, rec.Body.String())
	}
	var views []sharedMailboxView
	_ = json.Unmarshal(rec.Body.Bytes(), &views)
	if len(views) != 1 || views[0].Email != "support" || len(views[0].Members) != 1 || views[0].Members[0] != "username" {
		t.Fatalf("list = %+v, want one shared mailbox with member username", views)
	}
	if views[0].CreatedBy != "admin" {
		t.Errorf("created_by = %q, want admin", views[0].CreatedBy)
	}
}

func TestDeleteSharedMailboxRevokesAppPasswordAndCascades(t *testing.T) {
	h, srv, appMock := newSharedMailboxTestServer(t)
	adminToken := loginToken(t, h)
	id := createSharedMailbox(t, h, adminToken)

	appMock.mu.Lock()
	_, hadAppPw := appMock.byName["mailfold-shared:support"]
	appMock.mu.Unlock()
	if !hadAppPw {
		t.Fatal("expected a mailcow app-password to have been minted for the shared mailbox")
	}

	rec := doJSON(t, h, http.MethodDelete, "/api/shared-mailboxes", adminToken, map[string]string{"id": "999999"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete unknown shared mailbox = %d, want 404: %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodDelete, "/api/shared-mailboxes", adminToken, map[string]string{"id": strconv.FormatInt(id, 10)})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete shared mailbox = %d: %s", rec.Code, rec.Body.String())
	}

	appMock.mu.Lock()
	_, stillThere := appMock.byName["mailfold-shared:support"]
	appMock.mu.Unlock()
	if stillThere {
		t.Error("expected the mailcow app-password to be revoked after deleting the shared mailbox")
	}
	if _, ok, err := srv.sharedMailboxes.GetMailbox(id); err != nil || ok {
		t.Errorf("GetMailbox after delete: ok=%v err=%v, want gone", ok, err)
	}
}

func TestRemoveSharedMailboxMember(t *testing.T) {
	h, srv, _ := newSharedMailboxTestServer(t)
	adminToken := loginToken(t, h)
	id := createSharedMailbox(t, h, adminToken)

	rec := doJSON(t, h, http.MethodDelete, "/api/shared-mailboxes/members", adminToken, sharedMailboxMemberRequest{MailboxID: id, Email: "username"})
	if rec.Code != http.StatusOK {
		t.Fatalf("remove member = %d: %s", rec.Code, rec.Body.String())
	}
	members, err := srv.sharedMailboxes.ListMembers(id)
	if err != nil || len(members) != 0 {
		t.Fatalf("ListMembers after removal = %v, err=%v, want empty", members, err)
	}
}

func TestSharedMailboxAdminRoutesRequireAuth(t *testing.T) {
	h, _, _ := newSharedMailboxTestServer(t)
	rec := doJSON(t, h, http.MethodGet, "/api/shared-mailboxes", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list = %d, want 401", rec.Code)
	}
}

func TestCreateSharedMailboxRequiresEmail(t *testing.T) {
	h, _, _ := newSharedMailboxTestServer(t)
	adminToken := loginToken(t, h)
	rec := doJSON(t, h, http.MethodPost, "/api/shared-mailboxes", adminToken, sharedMailboxRequest{Email: "  "})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create with blank email = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteSharedMailboxInvalidID(t *testing.T) {
	h, _, _ := newSharedMailboxTestServer(t)
	adminToken := loginToken(t, h)
	rec := doJSON(t, h, http.MethodDelete, "/api/shared-mailboxes", adminToken, map[string]string{"id": "not-a-number"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("delete with invalid id = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestAddSharedMailboxMemberValidation(t *testing.T) {
	h, _, _ := newSharedMailboxTestServer(t)
	adminToken := loginToken(t, h)
	id := createSharedMailbox(t, h, adminToken)

	rec := doJSON(t, h, http.MethodPost, "/api/shared-mailboxes/members", adminToken, sharedMailboxMemberRequest{MailboxID: 0, Email: "bob@example.com"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("add member with no mailbox_id = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/api/shared-mailboxes/members", adminToken, sharedMailboxMemberRequest{MailboxID: id, Email: ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("add member with no email = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/api/shared-mailboxes/members", adminToken, sharedMailboxMemberRequest{MailboxID: 999999, Email: "bob@example.com"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("add member to unknown mailbox = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

func TestRemoveSharedMailboxMemberValidation(t *testing.T) {
	h, _, _ := newSharedMailboxTestServer(t)
	adminToken := loginToken(t, h)
	rec := doJSON(t, h, http.MethodDelete, "/api/shared-mailboxes/members", adminToken, sharedMailboxMemberRequest{MailboxID: 0, Email: ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("remove member with no mailbox_id/email = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

// TestCreateSharedMailboxAppPasswordMintFailure covers the compensating-delete
// path when mailcow's app-password id cannot be confirmed after creation —
// appMock.noReturn simulates the same "lost id" scenario apikeys_test.go
// already exercises for the analogous API-key mint path.
func TestCreateSharedMailboxAppPasswordMintFailure(t *testing.T) {
	h, _, appMock := newSharedMailboxTestServer(t)
	appMock.noReturn = true
	adminToken := loginToken(t, h)

	rec := doJSON(t, h, http.MethodPost, "/api/shared-mailboxes", adminToken, sharedMailboxRequest{Email: "support"})
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("create when app-password id cannot be confirmed = %d, want 502: %s", rec.Code, rec.Body.String())
	}
}
