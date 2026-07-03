package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
)

// stubResolver is a dnsResolver whose answers are fixed per test, so the DNS
// checks can be exercised without touching the live network.
type stubResolver struct {
	mx   map[string][]*net.MX
	txt  map[string][]string
	host map[string][]string
}

func (s *stubResolver) LookupMX(_ context.Context, name string) ([]*net.MX, error) {
	if v, ok := s.mx[name]; ok {
		return v, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: name}
}
func (s *stubResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	if v, ok := s.txt[name]; ok {
		return v, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: name}
}
func (s *stubResolver) LookupHost(_ context.Context, name string) ([]string, error) {
	if v, ok := s.host[name]; ok {
		return v, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: name}
}

// withStubResolver swaps the package-level resolver for the duration of a test.
func withStubResolver(t *testing.T, s *stubResolver) {
	t.Helper()
	prev := lookupResolver
	lookupResolver = s
	t.Cleanup(func() { lookupResolver = prev })
}

const dkimBody = `{"dkim_txt":"v=DKIM1; k=rsa; p=ABCDEF123456","dkim_selector":"dkim","length":2048}`

func TestDomainDNSAllGood(t *testing.T) {
	withStubResolver(t, &stubResolver{
		mx:   map[string][]*net.MX{"acme.io": {{Host: "mail.acme.io.", Pref: 10}}},
		host: map[string][]string{"mail.acme.io": {"203.0.113.24"}},
		txt: map[string][]string{
			"acme.io":                 {"v=spf1 mx ~all"},
			"dkim._domainkey.acme.io": {"v=DKIM1; k=rsa; p=ABCDEF123456"},
			"_dmarc.acme.io":          {"v=DMARC1; p=quarantine; rua=mailto:d@acme.io"},
		},
	})
	h := newAPIWithServerName(t, mockMailcow(t, 0, dkimBody).URL, "mail.acme.io")
	token := loginToken(t, h)

	rec := do(h, http.MethodGet, "/api/domains/acme.io/dns", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var out dnsCheck
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.SummaryOK {
		t.Fatalf("want all-good summary, got %+v", out)
	}
	if len(out.Records) != 5 {
		t.Fatalf("want 5 records, got %d", len(out.Records))
	}
	for _, r := range out.Records {
		if r.Status != "ok" {
			t.Errorf("record %s not ok: %+v", r.Type, r)
		}
	}
}

func TestDomainDNSMissingAndMismatch(t *testing.T) {
	withStubResolver(t, &stubResolver{
		// MX points at the wrong host -> mismatch; no A record -> missing;
		// no SPF/DMARC -> missing; DKIM TXT differs -> mismatch.
		mx: map[string][]*net.MX{"acme.io": {{Host: "mx.wrong.example.", Pref: 10}}},
		txt: map[string][]string{
			"dkim._domainkey.acme.io": {"v=DKIM1; k=rsa; p=DIFFERENTKEY"},
		},
	})
	h := newAPIWithServerName(t, mockMailcow(t, 0, dkimBody).URL, "mail.acme.io")
	token := loginToken(t, h)

	rec := do(h, http.MethodGet, "/api/domains/acme.io/dns", token, "")
	var out dnsCheck
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.SummaryOK {
		t.Fatalf("expected a bad summary, got %+v", out)
	}
	byType := map[string]dnsRecord{}
	for _, r := range out.Records {
		byType[r.Type] = r
	}
	if byType["MX"].Status != "mismatch" || byType["MX"].Found == "" {
		t.Errorf("MX = %+v", byType["MX"])
	}
	if byType["A"].Status != "missing" {
		t.Errorf("A = %+v", byType["A"])
	}
	if byType["SPF"].Status != "missing" {
		t.Errorf("SPF = %+v", byType["SPF"])
	}
	if byType["DKIM"].Status != "mismatch" {
		t.Errorf("DKIM = %+v", byType["DKIM"])
	}
	if byType["DMARC"].Status != "missing" {
		t.Errorf("DMARC = %+v", byType["DMARC"])
	}

	// Unauthenticated -> 401.
	if r2 := do(h, http.MethodGet, "/api/domains/acme.io/dns", "", ""); r2.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", r2.Code)
	}
}

func TestDomainDNSNoDkimKey(t *testing.T) {
	// mailcow returns an empty array when no DKIM key exists for the domain.
	withStubResolver(t, &stubResolver{})
	h := newAPIWithServerName(t, mockMailcow(t, 0, "[]").URL, "mail.acme.io")
	token := loginToken(t, h)

	rec := do(h, http.MethodGet, "/api/domains/acme.io/dns", token, "")
	var out dnsCheck
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	for _, r := range out.Records {
		if r.Type == "DKIM" && r.Status != "missing" {
			t.Errorf("DKIM without a key should be missing: %+v", r)
		}
	}
}

func TestDkimPubKeyAndShorten(t *testing.T) {
	if got := dkimPubKey("v=DKIM1; k=rsa; p=ABC DEF"); got != "ABCDEF" {
		t.Errorf("dkimPubKey = %q", got)
	}
	if got := dkimPubKey("v=DKIM1; k=rsa"); got != "" {
		t.Errorf("dkimPubKey with no p= = %q", got)
	}
	long := "0123456789012345678901234567890123456789012345678901234567890123456789"
	if got := shorten(long); len(got) >= len(long) {
		t.Errorf("shorten did not trim: %q", got)
	}
	if got := shorten("short"); got != "short" {
		t.Errorf("shorten should not touch short values: %q", got)
	}
}
