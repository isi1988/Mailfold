package mailcow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient returns a client wired to a mock mailcow server that returns
// canned success responses for every endpoint.
func newTestClient(t *testing.T) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "testkey" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
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
	return NewClient(srv.URL, "testkey", false)
}

func TestClientGetters(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	getters := []struct {
		name string
		fn   func() error
	}{
		{"Domains", func() error { _, err := c.Domains(ctx); return err }},
		{"Mailboxes", func() error { _, err := c.Mailboxes(ctx); return err }},
		{"Aliases", func() error { _, err := c.Aliases(ctx); return err }},
		{"SyncJobs", func() error { _, err := c.SyncJobs(ctx); return err }},
		{"MailQueue", func() error { _, err := c.MailQueue(ctx); return err }},
		{"Quarantine", func() error { _, err := c.Quarantine(ctx); return err }},
		{"Fail2Ban", func() error { _, err := c.Fail2Ban(ctx); return err }},
		{"Containers", func() error { _, err := c.Containers(ctx); return err }},
		{"Version", func() error { _, err := c.Version(ctx); return err }},
		{"Vmail", func() error { _, err := c.Vmail(ctx); return err }},
		{"DKIM", func() error { _, err := c.DKIM(ctx, "example.com"); return err }},
		{"Logs", func() error { _, err := c.Logs(ctx, "postfix", 50); return err }},
		{"PolicyAllow", func() error { _, err := c.PolicyAllow(ctx, "example.com"); return err }},
		{"PolicyDeny", func() error { _, err := c.PolicyDeny(ctx, "example.com"); return err }},
	}
	for _, g := range getters {
		if err := g.fn(); err != nil {
			t.Errorf("%s: %v", g.name, err)
		}
	}
}

func TestClientActions(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	attr := map[string]any{"x": "y"}
	items := []string{"a", "b"}
	actions := []struct {
		name string
		fn   func() ([]ActionResult, error)
	}{
		{"AddDomain", func() ([]ActionResult, error) { return c.AddDomain(ctx, attr) }},
		{"EditDomain", func() ([]ActionResult, error) { return c.EditDomain(ctx, items, attr) }},
		{"DeleteDomain", func() ([]ActionResult, error) { return c.DeleteDomain(ctx, items) }},
		{"AddMailbox", func() ([]ActionResult, error) { return c.AddMailbox(ctx, attr) }},
		{"EditMailbox", func() ([]ActionResult, error) { return c.EditMailbox(ctx, items, attr) }},
		{"DeleteMailbox", func() ([]ActionResult, error) { return c.DeleteMailbox(ctx, items) }},
		{"AddAlias", func() ([]ActionResult, error) { return c.AddAlias(ctx, attr) }},
		{"EditAlias", func() ([]ActionResult, error) { return c.EditAlias(ctx, items, attr) }},
		{"DeleteAlias", func() ([]ActionResult, error) { return c.DeleteAlias(ctx, items) }},
		{"AddDKIM", func() ([]ActionResult, error) { return c.AddDKIM(ctx, attr) }},
		{"DeleteDKIM", func() ([]ActionResult, error) { return c.DeleteDKIM(ctx, items) }},
		{"AddSyncJob", func() ([]ActionResult, error) { return c.AddSyncJob(ctx, attr) }},
		{"EditSyncJob", func() ([]ActionResult, error) { return c.EditSyncJob(ctx, items, attr) }},
		{"DeleteSyncJob", func() ([]ActionResult, error) { return c.DeleteSyncJob(ctx, items) }},
		{"FlushQueue", func() ([]ActionResult, error) { return c.FlushQueue(ctx) }},
		{"EditFail2Ban", func() ([]ActionResult, error) { return c.EditFail2Ban(ctx, attr) }},
		{"DeleteQuarantine", func() ([]ActionResult, error) { return c.DeleteQuarantine(ctx, items) }},
		{"AddPolicy", func() ([]ActionResult, error) { return c.AddPolicy(ctx, attr) }},
		{"DeletePolicy", func() ([]ActionResult, error) { return c.DeletePolicy(ctx, items) }},
	}
	for _, a := range actions {
		res, err := a.fn()
		if err != nil {
			t.Errorf("%s: %v", a.name, err)
			continue
		}
		if ok, _ := ResultsOK(res); !ok {
			t.Errorf("%s: expected successful results", a.name)
		}
	}
}

func TestResultsOK(t *testing.T) {
	if ok, _ := ResultsOK([]ActionResult{{Type: "success"}, {Type: "info"}}); !ok {
		t.Error("expected ok for success/info")
	}
	ok, msg := ResultsOK([]ActionResult{{Type: "danger", Msg: json.RawMessage(`"boom"`)}})
	if ok {
		t.Error("expected not ok for danger")
	}
	if !strings.Contains(msg, "boom") {
		t.Errorf("message=%q", msg)
	}
}

func TestClientErrorPaths(t *testing.T) {
	ctx := context.Background()

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	if _, err := NewClient(bad.URL, "k", false).Domains(ctx); err == nil {
		t.Error("expected error on HTTP 500")
	}

	badJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "not json")
	}))
	defer badJSON.Close()
	if _, err := NewClient(badJSON.URL, "k", false).Domains(ctx); err == nil {
		t.Error("expected decode error")
	}

	if _, err := NewClient("http://127.0.0.1:0", "k", true).Domains(ctx); err == nil {
		t.Error("expected transport error")
	}

	// A channel cannot be marshaled to JSON: exercises the post marshal error.
	if _, err := newTestClient(t).AddDomain(ctx, make(chan int)); err == nil {
		t.Error("expected marshal error")
	}
}

func TestClientAllMethodsErrorPaths(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	c := NewClient(bad.URL, "k", false)
	ctx := context.Background()
	attr := map[string]any{"x": "y"}
	items := []string{"a"}
	calls := []func() error{
		func() error { _, e := c.Domains(ctx); return e },
		func() error { _, e := c.AddDomain(ctx, attr); return e },
		func() error { _, e := c.EditDomain(ctx, items, attr); return e },
		func() error { _, e := c.DeleteDomain(ctx, items); return e },
		func() error { _, e := c.Mailboxes(ctx); return e },
		func() error { _, e := c.AddMailbox(ctx, attr); return e },
		func() error { _, e := c.EditMailbox(ctx, items, attr); return e },
		func() error { _, e := c.DeleteMailbox(ctx, items); return e },
		func() error { _, e := c.Aliases(ctx); return e },
		func() error { _, e := c.AddAlias(ctx, attr); return e },
		func() error { _, e := c.EditAlias(ctx, items, attr); return e },
		func() error { _, e := c.DeleteAlias(ctx, items); return e },
		func() error { _, e := c.DKIM(ctx, "d"); return e },
		func() error { _, e := c.AddDKIM(ctx, attr); return e },
		func() error { _, e := c.DeleteDKIM(ctx, items); return e },
		func() error { _, e := c.SyncJobs(ctx); return e },
		func() error { _, e := c.AddSyncJob(ctx, attr); return e },
		func() error { _, e := c.EditSyncJob(ctx, items, attr); return e },
		func() error { _, e := c.DeleteSyncJob(ctx, items); return e },
		func() error { _, e := c.MailQueue(ctx); return e },
		func() error { _, e := c.FlushQueue(ctx); return e },
		func() error { _, e := c.Logs(ctx, "postfix", 10); return e },
		func() error { _, e := c.Fail2Ban(ctx); return e },
		func() error { _, e := c.EditFail2Ban(ctx, attr); return e },
		func() error { _, e := c.Quarantine(ctx); return e },
		func() error { _, e := c.DeleteQuarantine(ctx, items); return e },
		func() error { _, e := c.PolicyAllow(ctx, "d"); return e },
		func() error { _, e := c.PolicyDeny(ctx, "d"); return e },
		func() error { _, e := c.AddPolicy(ctx, attr); return e },
		func() error { _, e := c.DeletePolicy(ctx, items); return e },
		func() error { _, e := c.Containers(ctx); return e },
		func() error { _, e := c.Version(ctx); return e },
		func() error { _, e := c.Vmail(ctx); return e },
		func() error { _, e := c.DomainAdmins(ctx); return e },
		func() error { _, e := c.AddDomainAdmin(ctx, attr); return e },
		func() error { _, e := c.EditDomainAdmin(ctx, items, attr); return e },
		func() error { _, e := c.DeleteDomainAdmin(ctx, items); return e },
		func() error { _, e := c.Resources(ctx); return e },
		func() error { _, e := c.AddResource(ctx, attr); return e },
		func() error { _, e := c.EditResource(ctx, items, attr); return e },
		func() error { _, e := c.DeleteResource(ctx, items); return e },
		func() error { _, e := c.AppPasswords(ctx, "u@example.com"); return e },
		func() error { _, e := c.AddAppPassword(ctx, attr); return e },
		func() error { _, e := c.DeleteAppPassword(ctx, items); return e },
		func() error { _, e := c.OAuth2Clients(ctx); return e },
		func() error { _, e := c.AddOAuth2Client(ctx, attr); return e },
		func() error { _, e := c.DeleteOAuth2Client(ctx, items); return e },
		func() error { _, e := c.ForwardingHosts(ctx); return e },
		func() error { _, e := c.AddForwardingHost(ctx, attr); return e },
		func() error { _, e := c.DeleteForwardingHost(ctx, items); return e },
		func() error { _, e := c.Transports(ctx); return e },
		func() error { _, e := c.AddTransport(ctx, attr); return e },
		func() error { _, e := c.EditTransport(ctx, items, attr); return e },
		func() error { _, e := c.DeleteTransport(ctx, items); return e },
		func() error { _, e := c.Relayhosts(ctx); return e },
		func() error { _, e := c.AddRelayhost(ctx, attr); return e },
		func() error { _, e := c.EditRelayhost(ctx, items, attr); return e },
		func() error { _, e := c.DeleteRelayhost(ctx, items); return e },
		func() error { _, e := c.TLSPolicies(ctx); return e },
		func() error { _, e := c.AddTLSPolicy(ctx, attr); return e },
		func() error { _, e := c.DeleteTLSPolicy(ctx, items); return e },
		func() error { _, e := c.BCCMaps(ctx); return e },
		func() error { _, e := c.AddBCC(ctx, attr); return e },
		func() error { _, e := c.DeleteBCC(ctx, items); return e },
		func() error { _, e := c.RecipientMaps(ctx); return e },
		func() error { _, e := c.AddRecipientMap(ctx, attr); return e },
		func() error { _, e := c.DeleteRecipientMap(ctx, items); return e },
		func() error { _, e := c.Admins(ctx); return e },
		func() error { _, e := c.AddAdmin(ctx, attr); return e },
		func() error { _, e := c.EditAdmin(ctx, items, attr); return e },
		func() error { _, e := c.DeleteAdmin(ctx, items); return e },
		func() error { _, e := c.DomainTemplates(ctx); return e },
		func() error { _, e := c.AddDomainTemplate(ctx, attr); return e },
		func() error { _, e := c.EditDomainTemplate(ctx, items, attr); return e },
		func() error { _, e := c.DeleteDomainTemplate(ctx, items); return e },
		func() error { _, e := c.MailboxTemplates(ctx); return e },
		func() error { _, e := c.AddMailboxTemplate(ctx, attr); return e },
		func() error { _, e := c.EditMailboxTemplate(ctx, items, attr); return e },
		func() error { _, e := c.DeleteMailboxTemplate(ctx, items); return e },
		func() error { _, e := c.RspamdSettings(ctx); return e },
		func() error { _, e := c.AddRspamdSetting(ctx, attr); return e },
		func() error { _, e := c.DeleteRspamdSetting(ctx, items); return e },
		func() error { _, e := c.RateLimitMailboxes(ctx); return e },
		func() error { _, e := c.EditRateLimitMailbox(ctx, items, attr); return e },
		func() error { _, e := c.RateLimitDomains(ctx); return e },
		func() error { _, e := c.EditRateLimitDomain(ctx, items, attr); return e },
		func() error { _, e := c.EditPushover(ctx, items, attr); return e },
		func() error { _, e := c.Filters(ctx); return e },
		func() error { _, e := c.AddFilter(ctx, attr); return e },
		func() error { _, e := c.EditFilter(ctx, items, attr); return e },
		func() error { _, e := c.DeleteFilter(ctx, items); return e },
		func() error { _, e := c.TempAliases(ctx, "u@example.com"); return e },
		func() error { _, e := c.AddTempAlias(ctx, attr); return e },
	}
	for i, fn := range calls {
		if err := fn(); err == nil {
			t.Errorf("call %d: expected error against failing server", i)
		}
	}
}

func TestGetListEmptyObject(t *testing.T) {
	// mailcow returns "{}" (an object) instead of "[]" for empty lists; the
	// typed listers must treat that as an empty slice rather than a decode error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{}")
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "k", false)
	ctx := context.Background()

	if d, err := c.Domains(ctx); err != nil || len(d) != 0 {
		t.Errorf("Domains on {}: got %v, err %v; want empty slice", d, err)
	}
	if m, err := c.Mailboxes(ctx); err != nil || len(m) != 0 {
		t.Errorf("Mailboxes on {}: got %v, err %v; want empty slice", m, err)
	}
	if a, err := c.Aliases(ctx); err != nil || len(a) != 0 {
		t.Errorf("Aliases on {}: got %v, err %v; want empty slice", a, err)
	}
}

func TestGetListDecodeError(t *testing.T) {
	// A non-empty object that is not a list must surface a decode error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"unexpected":"object"}`)
	}))
	defer srv.Close()
	if _, err := NewClient(srv.URL, "k", false).Domains(context.Background()); err == nil {
		t.Error("expected a decode error for a non-empty, non-list object")
	}
}
