package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
)

// fakeFilterMailcow is a minimal, STATEFUL fake of mailcow's Sieve-filter
// endpoints (/api/v1/{get,add,edit,delete}/filter*). The shared mockMailcow
// helper always returns an empty list from GET regardless of prior writes,
// which is fine for handlers that only ever send one mutation per test, but
// the rule builder's own list/edit/delete logic needs a GET that actually
// reflects earlier creates — hence a dedicated fake with real, if tiny, state.
type fakeFilterMailcow struct {
	mu      sync.Mutex
	nextID  int
	filters map[int]mailcowFilter
}

func newFakeFilterMailcow(t *testing.T) *httptest.Server {
	t.Helper()
	f := &fakeFilterMailcow{nextID: 1, filters: map[int]mailcowFilter{}}
	srv := httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(srv.Close)
	return srv
}

func (f *fakeFilterMailcow) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	f.mu.Lock()
	defer f.mu.Unlock()

	switch r.URL.Path {
	case "/api/v1/get/filters/all":
		list := make([]mailcowFilter, 0, len(f.filters))
		for _, filt := range f.filters {
			list = append(list, filt)
		}
		_ = json.NewEncoder(w).Encode(list)

	case "/api/v1/add/filter":
		var attr struct {
			Username   string `json:"username"`
			ScriptDesc string `json:"script_desc"`
			ScriptData string `json:"script_data"`
			Active     string `json:"active"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &attr)
		id := f.nextID
		f.nextID++
		active := 0
		if attr.Active == "1" {
			active = 1
		}
		f.filters[id] = mailcowFilter{ID: id, Username: attr.Username, ScriptDesc: attr.ScriptDesc, ScriptData: attr.ScriptData, Active: active}
		_, _ = io.WriteString(w, `[{"type":"success","msg":["filter_added"]}]`)

	case "/api/v1/edit/filter":
		var req struct {
			Items []string `json:"items"`
			Attr  struct {
				ScriptDesc string `json:"script_desc"`
				ScriptData string `json:"script_data"`
				Active     string `json:"active"`
			} `json:"attr"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		for _, item := range req.Items {
			id, err := strconv.Atoi(item)
			if err != nil {
				continue
			}
			filt, ok := f.filters[id]
			if !ok {
				continue
			}
			filt.ScriptDesc = req.Attr.ScriptDesc
			filt.ScriptData = req.Attr.ScriptData
			filt.Active = 0
			if req.Attr.Active == "1" {
				filt.Active = 1
			}
			f.filters[id] = filt
		}
		_, _ = io.WriteString(w, `[{"type":"success","msg":["filter_edited"]}]`)

	case "/api/v1/delete/filter":
		var items []string
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &items)
		for _, item := range items {
			if id, err := strconv.Atoi(item); err == nil {
				delete(f.filters, id)
			}
		}
		_, _ = io.WriteString(w, `[{"type":"success","msg":["filter_deleted"]}]`)

	default:
		_, _ = io.WriteString(w, `[]`)
	}
}

// --- Signature ---

func TestWebmailSignatureRequiresDB(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{withIMAP: true}) // no DB
	tok := webmailToken(t, h)
	if rec := do(h, http.MethodGet, "/api/webmail/signature", tok, ""); rec.Code != http.StatusNotImplemented {
		t.Errorf("GET signature without DB = %d, want 501", rec.Code)
	}
	if rec := do(h, http.MethodPut, "/api/webmail/signature", tok, `{"signature":"x"}`); rec.Code != http.StatusNotImplemented {
		t.Errorf("PUT signature without DB = %d, want 501", rec.Code)
	}
}

func TestWebmailSignatureGetSet(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{withDB: true, withIMAP: true})
	tok := webmailToken(t, h)

	rec := do(h, http.MethodGet, "/api/webmail/signature", tok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET signature = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"signature":""`) {
		t.Errorf("expected an empty default signature, got %s", rec.Body.String())
	}

	rec = do(h, http.MethodPut, "/api/webmail/signature", tok, `{"signature":"Best,\nUsername"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT signature = %d: %s", rec.Code, rec.Body.String())
	}
	rec = do(h, http.MethodGet, "/api/webmail/signature", tok, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Best,") {
		t.Errorf("signature not persisted: %d %s", rec.Code, rec.Body.String())
	}
}

// --- Rules ---

func TestWebmailRulesCRUDAndOwnership(t *testing.T) {
	imapAddr := startMemIMAP(t)
	cfg := &config.Config{
		MailcowBaseURL:    newFakeFilterMailcow(t).URL,
		MailcowAPIKey:     "k",
		AdminUser:         "admin",
		AdminPassword:     "pw",
		SessionTTL:        time.Hour,
		WebmailSessionTTL: time.Hour,
		CORSOrigins:       []string{"*"},
		LoginRateMax:      1000,
		LoginRateWindow:   time.Minute,
		MaxBodyBytes:      1 << 20,
		IMAPAddr:          imapAddr,
		MailUseTLS:        false,
	}
	mc := mailcow.NewClient(cfg.MailcowBaseURL, cfg.MailcowAPIKey, false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	limiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	srv := NewServer(cfg, mc, authn, limiter, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := srv.Handler()
	tok := webmailToken(t, h)

	// Validation: unknown field, empty value/folder.
	if rec := do(h, http.MethodPost, "/api/webmail/rules", tok, `{"description":"d","field":"bogus","value":"v","target_folder":"F"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("create with bad field = %d, want 400", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/webmail/rules", tok, `{"description":"d","field":"from","value":"","target_folder":"F"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("create with empty value = %d, want 400", rec.Code)
	}

	// Create.
	rec := do(h, http.MethodPost, "/api/webmail/rules", tok, `{"description":"Newsletters","field":"from","value":"newsletter@","target_folder":"Archive","active":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create rule = %d: %s", rec.Code, rec.Body.String())
	}

	// List: exactly the one rule just created, correctly reconstructed.
	rec = do(h, http.MethodGet, "/api/webmail/rules", tok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list rules = %d", rec.Code)
	}
	var rules []webmailRule
	if err := json.Unmarshal(rec.Body.Bytes(), &rules); err != nil {
		t.Fatalf("decode rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d: %+v", len(rules), rules)
	}
	got := rules[0]
	if got.Description != "Newsletters" || got.Field != "from" || got.Value != "newsletter@" || got.TargetFolder != "Archive" || !got.Active {
		t.Errorf("unexpected rule round-trip: %+v", got)
	}

	// A second mailbox (session seeded directly, bypassing IMAP — see
	// seedNotifySender for the same pattern) must not see or be able to edit
	// or delete the first mailbox's rule.
	otherTok, _, err := srv.webmailSessions.Create("other@example.com", "pw")
	if err != nil {
		t.Fatalf("seed other session: %v", err)
	}
	if rec := do(h, http.MethodGet, "/api/webmail/rules", otherTok, ""); rec.Code != http.StatusOK {
		t.Fatalf("list rules (other) = %d", rec.Code)
	} else {
		var otherRules []webmailRule
		_ = json.Unmarshal(rec.Body.Bytes(), &otherRules)
		if len(otherRules) != 0 {
			t.Errorf("other mailbox should see no rules, got %+v", otherRules)
		}
	}
	id := strconv.Itoa(got.ID)
	editBody := `{"id":"` + id + `","description":"hijacked","field":"from","value":"x","target_folder":"y","active":true}`
	if rec := do(h, http.MethodPut, "/api/webmail/rules", otherTok, editBody); rec.Code != http.StatusNotFound {
		t.Errorf("edit someone else's rule = %d, want 404", rec.Code)
	}
	deleteBody := `{"id":"` + id + `"}`
	if rec := do(h, http.MethodDelete, "/api/webmail/rules", otherTok, deleteBody); rec.Code != http.StatusNotFound {
		t.Errorf("delete someone else's rule = %d, want 404", rec.Code)
	}

	// The owner can edit and delete it.
	editBody = `{"id":"` + id + `","description":"Renamed","field":"subject","value":"invoice","target_folder":"Finance","active":false}`
	if rec := do(h, http.MethodPut, "/api/webmail/rules", tok, editBody); rec.Code != http.StatusOK {
		t.Fatalf("edit own rule = %d: %s", rec.Code, rec.Body.String())
	}
	rec = do(h, http.MethodGet, "/api/webmail/rules", tok, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &rules)
	if len(rules) != 1 || rules[0].Description != "Renamed" || rules[0].Active {
		t.Errorf("edit did not persist: %+v", rules)
	}
	if rec := do(h, http.MethodDelete, "/api/webmail/rules", tok, deleteBody); rec.Code != http.StatusOK {
		t.Fatalf("delete own rule = %d: %s", rec.Code, rec.Body.String())
	}
	rec = do(h, http.MethodGet, "/api/webmail/rules", tok, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &rules)
	if len(rules) != 0 {
		t.Errorf("rule should be gone after delete, got %+v", rules)
	}
}

// --- Two-factor auth ---

func TestWebmailTOTPRequiresCipher(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{withDB: true, withIMAP: true}) // no enc key
	tok := webmailToken(t, h)
	if rec := do(h, http.MethodPost, "/api/webmail/2fa/enroll", tok, `{"current_password":"password"}`); rec.Code != http.StatusNotImplemented {
		t.Errorf("enroll without cipher = %d, want 501", rec.Code)
	}
}

func TestWebmailTOTPEnrollConfirmLoginDisable(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{withDB: true, withEncKey: true, withIMAP: true})
	tok := webmailToken(t, h)

	if rec := do(h, http.MethodGet, "/api/webmail/2fa/status", tok, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("status before enroll = %d %s", rec.Code, rec.Body.String())
	}

	// Wrong current (real IMAP) password blocks enrollment.
	if rec := do(h, http.MethodPost, "/api/webmail/2fa/enroll", tok, `{"current_password":"wrong"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("enroll wrong pw = %d, want 401", rec.Code)
	}

	rec := do(h, http.MethodPost, "/api/webmail/2fa/enroll", tok, `{"current_password":"password"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("enroll = %d: %s", rec.Code, rec.Body.String())
	}
	var enroll struct {
		Secret    string `json:"secret"`
		QRDataURI string `json:"qr_data_uri"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &enroll)
	if enroll.Secret == "" || !strings.HasPrefix(enroll.QRDataURI, "data:image/png;base64,") {
		t.Fatalf("unexpected enroll response: %+v", enroll)
	}

	// A wrong code does not confirm enrollment.
	if rec := do(h, http.MethodPost, "/api/webmail/2fa/confirm", tok, `{"code":"000000"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("confirm wrong code = %d, want 401", rec.Code)
	}

	rec = do(h, http.MethodPost, "/api/webmail/2fa/confirm", tok, fmt.Sprintf(`{"code":%q}`, totpCodeFor(t, enroll.Secret)))
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm = %d body=%s", rec.Code, rec.Body.String())
	}
	var confirmed struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &confirmed)
	if len(confirmed.RecoveryCodes) != 10 {
		t.Fatalf("want 10 recovery codes, got %d", len(confirmed.RecoveryCodes))
	}
	if rec := do(h, http.MethodGet, "/api/webmail/2fa/status", tok, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":true`) {
		t.Fatalf("status after confirm = %d %s", rec.Code, rec.Body.String())
	}

	// A fresh webmail login now requires the second factor.
	rec = do(h, http.MethodPost, "/api/webmail/login", "", `{"email":"username","password":"password"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login = %d", rec.Code)
	}
	var pending struct {
		Needs2FA     bool   `json:"needs_2fa"`
		PendingToken string `json:"pending_token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &pending)
	if !pending.Needs2FA || pending.PendingToken == "" {
		t.Fatalf("expected a pending 2FA challenge, got %+v", pending)
	}

	// A wrong code at the verify step is rejected, but does NOT burn the
	// pending token: the same token must still work with the correct code
	// afterward (regression test for a bug where any wrong attempt
	// permanently stranded the login).
	badBody := fmt.Sprintf(`{"pending_token":%q,"code":"000000"}`, pending.PendingToken)
	if rec := do(h, http.MethodPost, "/api/webmail/2fa/verify", "", badBody); rec.Code != http.StatusUnauthorized {
		t.Errorf("2fa verify wrong code = %d, want 401", rec.Code)
	}

	goodBody := fmt.Sprintf(`{"pending_token":%q,"code":%q}`, pending.PendingToken, totpCodeFor(t, enroll.Secret))
	rec = do(h, http.MethodPost, "/api/webmail/2fa/verify", "", goodBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("2fa verify after a prior wrong code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var sess struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sess)
	if sess.Token == "" {
		t.Fatal("expected a session token after 2fa verify")
	}
	// The freshly minted session actually works.
	if rec := do(h, http.MethodGet, "/api/webmail/folders", sess.Token, ""); rec.Code != http.StatusOK {
		t.Errorf("folders with post-2fa session = %d", rec.Code)
	}

	// The pending token is single-use.
	if rec := do(h, http.MethodPost, "/api/webmail/2fa/verify", "", goodBody); rec.Code != http.StatusUnauthorized {
		t.Errorf("reusing a pending token = %d, want 401", rec.Code)
	}

	// A recovery code also redeems a fresh pending login, and is itself
	// single-use afterwards.
	rec = do(h, http.MethodPost, "/api/webmail/login", "", `{"email":"username","password":"password"}`)
	_ = json.Unmarshal(rec.Body.Bytes(), &pending)
	recoveryBody := fmt.Sprintf(`{"pending_token":%q,"code":%q}`, pending.PendingToken, confirmed.RecoveryCodes[0])
	if rec := do(h, http.MethodPost, "/api/webmail/2fa/verify", "", recoveryBody); rec.Code != http.StatusOK {
		t.Fatalf("recovery code verify = %d, body=%s", rec.Code, rec.Body.String())
	}
	rec = do(h, http.MethodPost, "/api/webmail/login", "", `{"email":"username","password":"password"}`)
	_ = json.Unmarshal(rec.Body.Bytes(), &pending)
	reuseBody := fmt.Sprintf(`{"pending_token":%q,"code":%q}`, pending.PendingToken, confirmed.RecoveryCodes[0])
	if rec := do(h, http.MethodPost, "/api/webmail/2fa/verify", "", reuseBody); rec.Code != http.StatusUnauthorized {
		t.Errorf("reusing a spent recovery code = %d, want 401", rec.Code)
	}

	// Recovery codes can be regenerated.
	rec = do(h, http.MethodPost, "/api/webmail/2fa/recovery-codes", sess.Token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("recovery regenerate = %d", rec.Code)
	}
	var regenerated struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &regenerated)
	if len(regenerated.RecoveryCodes) != 10 {
		t.Fatalf("want 10 regenerated codes, got %d", len(regenerated.RecoveryCodes))
	}

	// Disabling requires the current (real) password and turns 2FA fully off.
	if rec := do(h, http.MethodPost, "/api/webmail/2fa/disable", sess.Token, `{"current_password":"wrong"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("disable wrong pw = %d, want 401", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/webmail/2fa/disable", sess.Token, `{"current_password":"password"}`); rec.Code != http.StatusOK {
		t.Fatalf("disable = %d", rec.Code)
	}
	rec = do(h, http.MethodPost, "/api/webmail/login", "", `{"email":"username","password":"password"}`)
	var direct struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &direct)
	if rec.Code != http.StatusOK || direct.Token == "" {
		t.Errorf("login should no longer require 2FA after disable, code=%d body=%s", rec.Code, rec.Body.String())
	}

	// Regenerating recovery codes once 2FA is off is rejected.
	if rec := do(h, http.MethodPost, "/api/webmail/2fa/recovery-codes", direct.Token, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("recovery regenerate after disable = %d, want 400", rec.Code)
	}
}
