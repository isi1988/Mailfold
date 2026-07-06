package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
	"github.com/isi1988/Mailfold/backend/internal/webmail"
)

// newSharedMailboxMailcowMock combines a fixed mailbox list (so
// handleCreateSharedMailbox's existence check has something to match) with
// appPwMock's app-password mint/list/revoke handling.
func newSharedMailboxMailcowMock(t *testing.T, username string) (*httptest.Server, *appPwMock) {
	t.Helper()
	appMock := &appPwMock{byName: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/get/mailbox/all", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]mailcow.Mailbox{{Username: username, Active: 1}})
	})
	mux.Handle("/", appMock.handler())
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, appMock
}

// newSharedMailboxTestServer builds a full Server with a database (so shared
// mailboxes and the admin cipher are both available) and a real in-memory
// IMAP backend (the memory package's fixed "username"/"password" account),
// so a member can log in for real while the shared mailbox itself ("support"
// below) exists only in the mocked mailcow mailbox list — mirroring
// newSSOTestServer's precedent exactly, including why a shared-mailbox
// session is verified via the session store rather than a live IMAP round
// trip (see TestSSOCallbackSuccess).
func newSharedMailboxTestServer(t *testing.T) (http.Handler, *Server, *appPwMock) {
	t.Helper()
	imapAddr := startMemIMAP(t)
	mcSrv, appMock := newSharedMailboxMailcowMock(t, "support")

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 5)
	}
	cfg := &config.Config{
		MailcowBaseURL: mcSrv.URL, MailcowAPIKey: "k",
		AdminUser: "admin", AdminPassword: "pw",
		SessionTTL: time.Hour, WebmailSessionTTL: time.Hour,
		CORSOrigins: []string{"*"}, LoginRateMax: 1000, LoginRateWindow: time.Minute,
		IMAPAddr: imapAddr, MailUseTLS: false,
		DBDriver: "sqlite", DBPath: t.TempDir() + "/shared.db",
		AdminEncKey: key,
	}
	mc := mailcow.NewClient(cfg.MailcowBaseURL, cfg.MailcowAPIKey, false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	limiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	srv := NewServer(cfg, mc, authn, limiter, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if srv.sharedMailboxes == nil || srv.adminCipher == nil {
		t.Fatal("expected shared mailboxes to be configured (db + admin cipher both present)")
	}
	return srv.Handler(), srv, appMock
}

// webmailLoginToken logs in as the memory IMAP backend's fixed
// "username"/"password" account and returns the resulting bearer token.
func webmailLoginToken(t *testing.T, h http.Handler) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": "username", "password": "password"})
	req := httptest.NewRequest(http.MethodPost, "/api/webmail/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("webmail login = %d, body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Token == "" {
		t.Fatal("webmail login returned no token")
	}
	return out.Token
}

func doJSON(t *testing.T, h http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// createSharedMailbox is the admin-side happy path used as setup by several
// tests below: create the shared mailbox for the fixed "support" address
// newSharedMailboxTestServer's mailcow mock recognises, and grant the fixed
// in-memory IMAP account "username" access to it, returning the new
// mailbox's id.
func createSharedMailbox(t *testing.T, h http.Handler, adminToken string) int64 {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/api/shared-mailboxes", adminToken, sharedMailboxRequest{Email: "support", DisplayName: "Support"})
	if rec.Code != http.StatusOK {
		t.Fatalf("create shared mailbox = %d, body=%s", rec.Code, rec.Body.String())
	}
	var view sharedMailboxView
	_ = json.Unmarshal(rec.Body.Bytes(), &view)
	rec = doJSON(t, h, http.MethodPost, "/api/shared-mailboxes/members", adminToken, sharedMailboxMemberRequest{MailboxID: view.ID, Email: "username"})
	if rec.Code != http.StatusOK {
		t.Fatalf("add member = %d, body=%s", rec.Code, rec.Body.String())
	}
	return view.ID
}

func TestWebmailSharedDisabledWithoutStore(t *testing.T) {
	h := newAPIWithIMAP(t, mockMailcow(t, 0, "").URL, startMemIMAP(t))
	token := webmailLoginToken(t, h)

	// Discovery reports an empty list rather than an error — nothing to
	// offer, matching the SSO provider-discovery precedent.
	rec := doJSON(t, h, http.MethodGet, "/api/webmail/shared", token, nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "[]\n" {
		t.Fatalf("shared mailboxes when disabled = %d %q, want 200 []", rec.Code, rec.Body.String())
	}
	// Activating a session, though, requires the store and reports 501.
	rec = doJSON(t, h, http.MethodPost, "/api/webmail/shared/1/session", token, nil)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("activate session when disabled = %d, want 501", rec.Code)
	}
}

func TestCreateSharedMailboxRequiresExistingActiveMailbox(t *testing.T) {
	h, _, _ := newSharedMailboxTestServer(t)
	adminToken := loginToken(t, h)

	rec := doJSON(t, h, http.MethodPost, "/api/shared-mailboxes", adminToken, sharedMailboxRequest{Email: "nobody@example.com"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create for a nonexistent mailbox = %d, want 400: %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/api/shared-mailboxes", adminToken, sharedMailboxRequest{Email: "support", DisplayName: "Support"})
	if rec.Code != http.StatusOK {
		t.Fatalf("create shared mailbox = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	// Creating the same shared mailbox twice is rejected.
	rec = doJSON(t, h, http.MethodPost, "/api/shared-mailboxes", adminToken, sharedMailboxRequest{Email: "support"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate create = %d, want 409: %s", rec.Code, rec.Body.String())
	}
}

func TestSharedMailboxMemberDiscoverActivateAndAssignmentNotes(t *testing.T) {
	h, srv, _ := newSharedMailboxTestServer(t)
	adminToken := loginToken(t, h)
	memberToken := webmailLoginToken(t, h)

	id := createSharedMailbox(t, h, adminToken)

	// The member can now see the shared mailbox in their own discovery list.
	rec := doJSON(t, h, http.MethodGet, "/api/webmail/shared", memberToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list shared mailboxes = %d: %s", rec.Code, rec.Body.String())
	}
	var opts []sharedMailboxOption
	_ = json.Unmarshal(rec.Body.Bytes(), &opts)
	if len(opts) != 1 || opts[0].Email != "support" || opts[0].ID != id {
		t.Fatalf("discovered shared mailboxes = %+v, want one entry for %q (id %d)", opts, "support", id)
	}

	// Activating it mints a real webmail session for "support", tagged with
	// ActingAs so it can be attributed back to the member (verified via the
	// session store directly rather than a live IMAP round trip — see
	// newSharedMailboxTestServer's doc comment for why).
	rec = doJSON(t, h, http.MethodPost, "/api/webmail/shared/"+strconv.FormatInt(id, 10)+"/session", memberToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("activate session = %d: %s", rec.Code, rec.Body.String())
	}
	var sess struct {
		Token string `json:"token"`
		Email string `json:"email"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sess)
	if sess.Email != "support" || sess.Token == "" {
		t.Fatalf("activated session = %+v, want email=support and a non-empty token", sess)
	}
	cred, ok := srv.webmailSessions.Get(sess.Token)
	if !ok || cred.Email != "support" || cred.ActingAs != "username" {
		t.Fatalf("minted session = %+v, %v, want email=support ActingAs=username", cred, ok)
	}

	sharedToken := sess.Token

	// Team member list, seen from inside the activated session.
	rec = doJSON(t, h, http.MethodGet, "/api/webmail/shared/members", sharedToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list members = %d: %s", rec.Code, rec.Body.String())
	}
	var members []string
	_ = json.Unmarshal(rec.Body.Bytes(), &members)
	if len(members) != 1 || members[0] != "username" {
		t.Fatalf("members = %v, want [username]", members)
	}

	// Assigning to a non-member is rejected.
	rec = doJSON(t, h, http.MethodPost, "/api/webmail/shared/assignment", sharedToken, setAssignmentRequest{Folder: "INBOX", UID: 1, AssignedTo: "stranger@example.com"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("assign to a non-member = %d, want 400: %s", rec.Code, rec.Body.String())
	}

	// Assign to the (only) member, then read it back.
	rec = doJSON(t, h, http.MethodPost, "/api/webmail/shared/assignment", sharedToken, setAssignmentRequest{Folder: "INBOX", UID: 1, AssignedTo: "username"})
	if rec.Code != http.StatusOK {
		t.Fatalf("set assignment = %d: %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/api/webmail/shared/assignment?folder=INBOX&uid=1", sharedToken, nil)
	var av assignmentView
	_ = json.Unmarshal(rec.Body.Bytes(), &av)
	if av.AssignedTo != "username" {
		t.Fatalf("assignment = %+v, want assigned_to=username", av)
	}

	// Clearing (empty assigned_to) removes it.
	rec = doJSON(t, h, http.MethodPost, "/api/webmail/shared/assignment", sharedToken, setAssignmentRequest{Folder: "INBOX", UID: 1, AssignedTo: ""})
	if rec.Code != http.StatusOK {
		t.Fatalf("clear assignment = %d: %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/api/webmail/shared/assignment?folder=INBOX&uid=1", sharedToken, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &av)
	if av.AssignedTo != "" {
		t.Fatalf("assignment after clear = %+v, want empty", av)
	}

	// Notes: add, list, then delete as the author.
	rec = doJSON(t, h, http.MethodPost, "/api/webmail/shared/notes", sharedToken, addNoteRequest{Folder: "INBOX", UID: 1, Body: "customer called back"})
	if rec.Code != http.StatusOK {
		t.Fatalf("add note = %d: %s", rec.Code, rec.Body.String())
	}
	var note noteView
	_ = json.Unmarshal(rec.Body.Bytes(), &note)
	if note.Author != "username" || note.Body != "customer called back" || note.ID == 0 {
		t.Fatalf("added note = %+v, want author=username", note)
	}

	rec = doJSON(t, h, http.MethodGet, "/api/webmail/shared/notes?folder=INBOX&uid=1", sharedToken, nil)
	var notes []noteView
	_ = json.Unmarshal(rec.Body.Bytes(), &notes)
	if len(notes) != 1 || notes[0].ID != note.ID {
		t.Fatalf("list notes = %+v, want one matching the added note", notes)
	}

	rec = doJSON(t, h, http.MethodDelete, "/api/webmail/shared/notes", sharedToken, map[string]int64{"id": note.ID})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete own note = %d: %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/api/webmail/shared/notes?folder=INBOX&uid=1", sharedToken, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &notes)
	if len(notes) != 0 {
		t.Fatalf("notes after delete = %+v, want empty", notes)
	}
}

func TestSharedMailboxNonMemberCannotActivateSession(t *testing.T) {
	h, _, _ := newSharedMailboxTestServer(t)
	adminToken := loginToken(t, h)
	memberToken := webmailLoginToken(t, h)

	// Create the shared mailbox but do NOT grant "username" access.
	rec := doJSON(t, h, http.MethodPost, "/api/shared-mailboxes", adminToken, sharedMailboxRequest{Email: "support"})
	var view sharedMailboxView
	_ = json.Unmarshal(rec.Body.Bytes(), &view)

	rec = doJSON(t, h, http.MethodPost, "/api/webmail/shared/"+strconv.FormatInt(view.ID, 10)+"/session", memberToken, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("activate session without membership = %d, want 403: %s", rec.Code, rec.Body.String())
	}
}

func TestSharedMailboxOnlyAuthorCanDeleteNote(t *testing.T) {
	h, srv, _ := newSharedMailboxTestServer(t)
	adminToken := loginToken(t, h)
	memberToken := webmailLoginToken(t, h)
	id := createSharedMailbox(t, h, adminToken)

	rec := doJSON(t, h, http.MethodPost, "/api/webmail/shared/"+strconv.FormatInt(id, 10)+"/session", memberToken, nil)
	var sess struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sess)

	rec = doJSON(t, h, http.MethodPost, "/api/webmail/shared/notes", sess.Token, addNoteRequest{Folder: "INBOX", UID: 1, Body: "note by username"})
	var note noteView
	_ = json.Unmarshal(rec.Body.Bytes(), &note)

	// A second member's session, minted directly against the session store
	// (there is only one real IMAP account in this test's fake backend, so a
	// second real login isn't available — the session store is the same
	// mechanism a real second login would populate).
	if err := srv.sharedMailboxes.AddMember(id, "bob@example.com", time.Now()); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	bobToken, _, err := srv.webmailSessions.CreateActingAs("support", "irrelevant", "bob@example.com")
	if err != nil {
		t.Fatalf("CreateActingAs: %v", err)
	}

	rec = doJSON(t, h, http.MethodDelete, "/api/webmail/shared/notes", bobToken, map[string]int64{"id": note.ID})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("delete someone else's note = %d, want 403: %s", rec.Code, rec.Body.String())
	}
}

// TestEnrichSharedFillsAssignmentAndNotes exercises enrichShared directly
// (rather than through handleWebmailMessages, which would need "support" to
// be a real IMAP account — the stock in-memory backend only ever ships the
// one fixed "username"/"password" mailbox, see newSharedMailboxTestServer's
// doc comment): it fabricates a request carrying shared-mailbox credentials
// exactly like requireWebmail would, and checks that headers matching an
// assignment or a note get filled in while an untouched message is left
// alone.
func TestEnrichSharedFillsAssignmentAndNotes(t *testing.T) {
	h, srv, _ := newSharedMailboxTestServer(t)
	adminToken := loginToken(t, h)
	id := createSharedMailbox(t, h, adminToken)

	if err := srv.sharedMailboxes.SetAssignment(id, "INBOX", 1, "username", "username", time.Now()); err != nil {
		t.Fatalf("SetAssignment: %v", err)
	}
	if _, err := srv.sharedMailboxes.AddNote(id, "INBOX", 1, "username", "hi", time.Now()); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	if _, err := srv.sharedMailboxes.AddNote(id, "INBOX", 1, "username", "hi again", time.Now()); err != nil {
		t.Fatalf("AddNote: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/webmail/messages", nil)
	cred := &webmail.Credentials{Email: "support", Password: "app-pw", ActingAs: "username"}
	req = req.WithContext(context.WithValue(req.Context(), webmailCtxKey, cred))

	msgs := []webmail.MessageHeader{{UID: 1}, {UID: 2}}
	srv.enrichShared(req, "INBOX", msgs)
	if msgs[0].AssignedTo != "username" || msgs[0].NotesCount != 2 {
		t.Errorf("msgs[0] = %+v, want AssignedTo=username NotesCount=2", msgs[0])
	}
	if msgs[1].AssignedTo != "" || msgs[1].NotesCount != 0 {
		t.Errorf("msgs[1] = %+v, want untouched (no assignment or notes)", msgs[1])
	}
}

// TestSharedMailboxEndpointsRequireASharedSession checks that every
// per-message endpoint reports 404 (not a shared mailbox, matching
// resolveSharedSession's doc comment on why it never leaks whether the
// address happens to be a plain mailbox) when called from an ordinary
// webmail session rather than an activated shared-mailbox one.
func TestSharedMailboxEndpointsRequireASharedSession(t *testing.T) {
	h, _, _ := newSharedMailboxTestServer(t)
	adminToken := loginToken(t, h)
	createSharedMailbox(t, h, adminToken)
	// "username" itself is a member, but this is its ORDINARY session — not
	// one activated via /api/webmail/shared/{id}/session — so from its own
	// mailbox's point of view it is not a shared mailbox at all.
	ordinaryToken := webmailLoginToken(t, h)

	for _, c := range []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"members", http.MethodGet, "/api/webmail/shared/members", nil},
		{"get assignment", http.MethodGet, "/api/webmail/shared/assignment?folder=INBOX&uid=1", nil},
		{"set assignment", http.MethodPost, "/api/webmail/shared/assignment", setAssignmentRequest{Folder: "INBOX", UID: 1, AssignedTo: "username"}},
		{"list notes", http.MethodGet, "/api/webmail/shared/notes?folder=INBOX&uid=1", nil},
		{"add note", http.MethodPost, "/api/webmail/shared/notes", addNoteRequest{Folder: "INBOX", UID: 1, Body: "x"}},
		{"delete note", http.MethodDelete, "/api/webmail/shared/notes", map[string]int64{"id": 1}},
	} {
		rec := doJSON(t, h, c.method, c.path, ordinaryToken, c.body)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s from an ordinary session = %d, want 404: %s", c.name, rec.Code, rec.Body.String())
		}
	}
}

func TestWebmailSharedSessionValidation(t *testing.T) {
	h, _, _ := newSharedMailboxTestServer(t)
	memberToken := webmailLoginToken(t, h)

	rec := doJSON(t, h, http.MethodPost, "/api/webmail/shared/not-a-number/session", memberToken, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("activate with invalid id = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/api/webmail/shared/999999/session", memberToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("activate unknown shared mailbox = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

func TestWebmailSharedAssignmentAndNotesValidation(t *testing.T) {
	h, _, _ := newSharedMailboxTestServer(t)
	adminToken := loginToken(t, h)
	memberToken := webmailLoginToken(t, h)
	id := createSharedMailbox(t, h, adminToken)
	rec := doJSON(t, h, http.MethodPost, "/api/webmail/shared/"+strconv.FormatInt(id, 10)+"/session", memberToken, nil)
	var sess struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sess)

	rec = doJSON(t, h, http.MethodGet, "/api/webmail/shared/assignment?folder=INBOX&uid=not-a-number", sess.Token, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("get assignment with invalid uid = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/api/webmail/shared/notes?folder=INBOX&uid=not-a-number", sess.Token, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("list notes with invalid uid = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/api/webmail/shared/notes", sess.Token, addNoteRequest{Folder: "INBOX", UID: 1, Body: "   "})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("add a blank note = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodDelete, "/api/webmail/shared/notes", sess.Token, map[string]int64{"id": 999999})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete a nonexistent note = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}
