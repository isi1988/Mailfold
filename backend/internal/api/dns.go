package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// dnsRecord is one expected DNS record and how the live zone compares to it.
type dnsRecord struct {
	Type   string `json:"type"`
	Host   string `json:"host"`
	Value  string `json:"value"`           // the record the operator should publish
	Status string `json:"status"`          // ok | missing | mismatch
	Found  string `json:"found,omitempty"` // what is actually published, when it differs
}

// dnsCheck is the result of verifying a domain's mail DNS.
type dnsCheck struct {
	Domain    string      `json:"domain"`
	Records   []dnsRecord `json:"records"`
	SummaryOK bool        `json:"summary_ok"`
	Summary   string      `json:"summary"`
}

// dnsResolver is the subset of *net.Resolver the checks use, so tests can supply
// a stub instead of hitting the live DNS.
type dnsResolver interface {
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
	LookupTXT(ctx context.Context, name string) ([]string, error)
	LookupHost(ctx context.Context, host string) ([]string, error)
}

// lookupResolver is swappable in tests; it defaults to the system resolver.
var lookupResolver dnsResolver = &net.Resolver{}

// registerDomainDNSRoutes wires the per-domain DNS verification endpoint.
func (s *Server) registerDomainDNSRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/domains/{domain}/dns", s.requireAuth(s.handleDomainDNS))
}

// handleDomainDNS builds the expected mail records for a domain and checks each
// against the live zone (MX, the mail host's A, SPF, DKIM and DMARC).
func (s *Server) handleDomainDNS(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	host := s.cfg.ServerName
	if host == "" {
		host = "mail." + domain
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	selector, dkimTxt := s.domainDKIM(ctx, domain)
	res := lookupResolver
	records := []dnsRecord{
		checkMX(ctx, res, domain, host),
		checkHostA(ctx, res, host),
		checkSPF(ctx, res, domain),
		checkDKIM(ctx, res, domain, selector, dkimTxt),
		checkDMARC(ctx, res, domain),
	}

	bad := 0
	for _, rec := range records {
		if rec.Status != "ok" {
			bad++
		}
	}
	out := dnsCheck{Domain: domain, Records: records, SummaryOK: bad == 0}
	if bad == 0 {
		out.Summary = "All mail records are in place."
	} else {
		out.Summary = fmt.Sprintf("%d of %d records need attention.", bad, len(records))
	}
	writeJSON(w, http.StatusOK, out)
}

// domainDKIM returns the DKIM selector host and expected TXT value from mailcow,
// or empty strings when no key exists.
func (s *Server) domainDKIM(ctx context.Context, domain string) (selector, txt string) {
	selector = "dkim._domainkey"
	raw, err := s.mc.DKIM(ctx, domain)
	if err != nil {
		return selector, ""
	}
	var d struct {
		DkimTxt      string `json:"dkim_txt"`
		DkimSelector string `json:"dkim_selector"`
	}
	if json.Unmarshal(raw, &d) != nil {
		return selector, ""
	}
	if d.DkimSelector != "" {
		selector = d.DkimSelector + "._domainkey"
	}
	return selector, d.DkimTxt
}

func checkMX(ctx context.Context, res dnsResolver, domain, host string) dnsRecord {
	rec := dnsRecord{Type: "MX", Host: "@", Value: "10 " + host + ".", Status: "missing"}
	mxs, err := res.LookupMX(ctx, domain)
	if err != nil || len(mxs) == 0 {
		rec.Found = "— no MX record —"
		return rec
	}
	var found []string
	for _, m := range mxs {
		found = append(found, fmt.Sprintf("%d %s", m.Pref, strings.TrimSuffix(m.Host, ".")))
		if strings.EqualFold(strings.TrimSuffix(m.Host, "."), host) {
			rec.Status = "ok"
			return rec
		}
	}
	rec.Status = "mismatch"
	rec.Found = strings.Join(found, ", ")
	return rec
}

func checkHostA(ctx context.Context, res dnsResolver, host string) dnsRecord {
	rec := dnsRecord{Type: "A", Host: host, Status: "missing"}
	addrs, err := res.LookupHost(ctx, host)
	var v4 []string
	for _, a := range addrs {
		if !strings.Contains(a, ":") {
			v4 = append(v4, a)
		}
	}
	if err != nil || len(v4) == 0 {
		rec.Value = "an A record for " + host
		rec.Found = "— does not resolve —"
		return rec
	}
	rec.Value = strings.Join(v4, ", ")
	rec.Status = "ok"
	return rec
}

func checkSPF(ctx context.Context, res dnsResolver, domain string) dnsRecord {
	rec := dnsRecord{Type: "SPF", Host: "@", Value: "v=spf1 mx ~all", Status: "missing"}
	for _, t := range lookupTXT(ctx, res, domain) {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(t)), "v=spf1") {
			rec.Status = "ok"
			return rec
		}
	}
	rec.Found = "— no SPF record —"
	return rec
}

func checkDKIM(ctx context.Context, res dnsResolver, domain, selector, expected string) dnsRecord {
	rec := dnsRecord{Type: "DKIM", Host: selector, Value: shorten(expected), Status: "missing"}
	if expected == "" {
		rec.Value = "— generate a DKIM key —"
		return rec
	}
	joined := strings.Join(lookupTXT(ctx, res, selector+"."+domain), "")
	found := dkimPubKey(joined)
	if found == "" {
		rec.Found = "— no DKIM record —"
		return rec
	}
	if found == dkimPubKey(expected) {
		rec.Status = "ok"
		return rec
	}
	rec.Status = "mismatch"
	rec.Found = shorten(joined)
	return rec
}

func checkDMARC(ctx context.Context, res dnsResolver, domain string) dnsRecord {
	rec := dnsRecord{Type: "DMARC", Host: "_dmarc", Value: "v=DMARC1; p=quarantine", Status: "missing"}
	for _, t := range lookupTXT(ctx, res, "_dmarc."+domain) {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(t)), "v=dmarc1") {
			rec.Status = "ok"
			return rec
		}
	}
	rec.Found = "— no DMARC record —"
	return rec
}

// lookupTXT resolves TXT records, returning nil on any error.
func lookupTXT(ctx context.Context, res dnsResolver, name string) []string {
	txts, err := res.LookupTXT(ctx, name)
	if err != nil {
		return nil
	}
	return txts
}

// dkimPubKey extracts the p= public key from a DKIM TXT value, ignoring spaces.
func dkimPubKey(txt string) string {
	for _, part := range strings.Split(txt, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "p=") {
			return strings.ReplaceAll(part[2:], " ", "")
		}
	}
	return ""
}

// shorten trims a long value (e.g. a DKIM key) for display.
func shorten(v string) string {
	if len(v) <= 60 {
		return v
	}
	return v[:40] + "…" + v[len(v)-12:]
}
