package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// bulkMailboxMock is a stateful stand-in for mailcow's add/mailbox endpoint: it
// records every attr map it receives and fails (mailcow "danger" result) any
// local_part in failLocalParts, succeeding for everything else.
type bulkMailboxMock struct {
	calls          []map[string]any
	failLocalParts map[string]bool
}

func (m *bulkMailboxMock) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/add/mailbox", func(w http.ResponseWriter, r *http.Request) {
		var attr map[string]any
		_ = json.NewDecoder(r.Body).Decode(&attr)
		m.calls = append(m.calls, attr)
		lp, _ := attr["local_part"].(string)
		if m.failLocalParts[lp] {
			_, _ = io.WriteString(w, `[{"type":"danger","msg":"mailbox_already_exists"}]`)
			return
		}
		_, _ = io.WriteString(w, `[{"type":"success","msg":"mailbox_added"}]`)
	})
	return mux
}

func newMailboxBulkServer(t *testing.T, mock *bulkMailboxMock) http.Handler {
	t.Helper()
	mcSrv := httptest.NewServer(mock.handler())
	t.Cleanup(mcSrv.Close)
	return newAPI(t, mcSrv.URL, []string{"*"})
}

func TestMailboxBulkImportSuccess(t *testing.T) {
	mock := &bulkMailboxMock{failLocalParts: map[string]bool{}}
	h := newMailboxBulkServer(t, mock)
	token := loginToken(t, h)

	csvBody := "local_part,domain,password,name\n" +
		"alice,example.com,supersecret1,Alice A\n" +
		"bob,example.com,supersecret2,Bob B\n"
	body, _ := json.Marshal(map[string]string{"csv": csvBody})
	rec := do(h, http.MethodPost, "/api/mailboxes/bulk", token, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("bulk import = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp mailboxBulkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Created != 2 || resp.Failed != 0 {
		t.Fatalf("want created=2 failed=0, got created=%d failed=%d", resp.Created, resp.Failed)
	}
	if len(mock.calls) != 2 {
		t.Fatalf("want 2 upstream calls, got %d", len(mock.calls))
	}
	if mock.calls[0]["local_part"] != "alice" || mock.calls[0]["domain"] != "example.com" || mock.calls[0]["password"] != "supersecret1" || mock.calls[0]["password2"] != "supersecret1" {
		t.Errorf("unexpected attr for row 1: %+v", mock.calls[0])
	}
	// Default quota (3GB -> 3072MB) and active applied when omitted.
	if mock.calls[0]["quota"] != float64(3072) {
		t.Errorf("want default quota 3072, got %v", mock.calls[0]["quota"])
	}
	if mock.calls[0]["active"] != "1" {
		t.Errorf("want default active=1, got %v", mock.calls[0]["active"])
	}
}

func TestMailboxBulkImportPartialFailureUpstream(t *testing.T) {
	mock := &bulkMailboxMock{failLocalParts: map[string]bool{"dup": true}}
	h := newMailboxBulkServer(t, mock)
	token := loginToken(t, h)

	csvBody := "local_part,domain,password\n" +
		"dup,example.com,supersecret1\n" +
		"fresh,example.com,supersecret2\n"
	body, _ := json.Marshal(map[string]string{"csv": csvBody})
	rec := do(h, http.MethodPost, "/api/mailboxes/bulk", token, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("bulk import = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp mailboxBulkResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Created != 1 || resp.Failed != 1 {
		t.Fatalf("want created=1 failed=1, got created=%d failed=%d", resp.Created, resp.Failed)
	}
	if resp.Results[0].OK || resp.Results[0].Error == "" {
		t.Errorf("row 1 (dup) should fail with a message: %+v", resp.Results[0])
	}
	if !resp.Results[1].OK || resp.Results[1].Error != "" {
		t.Errorf("row 2 (fresh) should succeed with no error: %+v", resp.Results[1])
	}
}

func TestMailboxBulkImportRowValidation(t *testing.T) {
	mock := &bulkMailboxMock{failLocalParts: map[string]bool{}}
	h := newMailboxBulkServer(t, mock)
	token := loginToken(t, h)

	csvBody := "local_part,domain,password\n" +
		",example.com,supersecret1\n" + // missing local_part
		"noshortpw,example.com,short\n" + // password too short
		"good,example.com,supersecret2\n"
	body, _ := json.Marshal(map[string]string{"csv": csvBody})
	rec := do(h, http.MethodPost, "/api/mailboxes/bulk", token, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("bulk import = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp mailboxBulkResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Created != 1 || resp.Failed != 2 {
		t.Fatalf("want created=1 failed=2, got created=%d failed=%d", resp.Created, resp.Failed)
	}
	// Invalid rows must never reach mailcow.
	if len(mock.calls) != 1 {
		t.Fatalf("want exactly 1 upstream call (only the valid row), got %d", len(mock.calls))
	}
}

func TestMailboxBulkImportCustomColumns(t *testing.T) {
	mock := &bulkMailboxMock{failLocalParts: map[string]bool{}}
	h := newMailboxBulkServer(t, mock)
	token := loginToken(t, h)

	// Header casing/whitespace and column order should not matter, and
	// quota_gb/active should override the defaults.
	csvBody := " Local_Part , Domain,quota_gb,ACTIVE,password\n" +
		"custom,example.com,10,0,supersecret1\n"
	body, _ := json.Marshal(map[string]string{"csv": csvBody})
	rec := do(h, http.MethodPost, "/api/mailboxes/bulk", token, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("bulk import = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(mock.calls) != 1 {
		t.Fatalf("want 1 upstream call, got %d", len(mock.calls))
	}
	if mock.calls[0]["quota"] != float64(10*1024) {
		t.Errorf("want quota 10240, got %v", mock.calls[0]["quota"])
	}
	if mock.calls[0]["active"] != "0" {
		t.Errorf("want active=0, got %v", mock.calls[0]["active"])
	}
}

func TestMailboxBulkImportBadRequests(t *testing.T) {
	mock := &bulkMailboxMock{failLocalParts: map[string]bool{}}
	h := newMailboxBulkServer(t, mock)
	token := loginToken(t, h)

	cases := []struct {
		name string
		csv  string
	}{
		{"empty csv", ""},
		{"header only, no data rows", "local_part,domain,password\n"},
		{"ragged row", "local_part,domain,password\nfoo,example.com\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"csv": c.csv})
			rec := do(h, http.MethodPost, "/api/mailboxes/bulk", token, string(body))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s: want 400, got %d (body=%s)", c.name, rec.Code, rec.Body.String())
			}
		})
	}

	if rec := do(h, http.MethodPost, "/api/mailboxes/bulk", token, "{bad"); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed JSON body: want 400, got %d", rec.Code)
	}
}

func TestMailboxBulkImportTooManyRows(t *testing.T) {
	mock := &bulkMailboxMock{failLocalParts: map[string]bool{}}
	h := newMailboxBulkServer(t, mock)
	token := loginToken(t, h)

	var sb strings.Builder
	sb.WriteString("local_part,domain,password\n")
	for i := 0; i < maxBulkMailboxRows+1; i++ {
		fmt.Fprintf(&sb, "user%d,example.com,supersecret1\n", i)
	}
	body, _ := json.Marshal(map[string]string{"csv": sb.String()})
	rec := do(h, http.MethodPost, "/api/mailboxes/bulk", token, string(body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for too many rows, got %d", rec.Code)
	}
	if len(mock.calls) != 0 {
		t.Errorf("an over-limit import should not call mailcow at all, got %d calls", len(mock.calls))
	}
}

func TestMailboxBulkImportRequiresAuth(t *testing.T) {
	mock := &bulkMailboxMock{failLocalParts: map[string]bool{}}
	h := newMailboxBulkServer(t, mock)
	body, _ := json.Marshal(map[string]string{"csv": "local_part,domain,password\nx,example.com,supersecret1\n"})
	if rec := do(h, http.MethodPost, "/api/mailboxes/bulk", "", string(body)); rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401 without a token, got %d", rec.Code)
	}
}
