package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

// validLogServices is the allowlist of mailcow log services that may be queried.
// It exists as a security and correctness guard: the requested service name is
// interpolated into the upstream mailcow log request, so restricting it to a
// fixed set of known-good values prevents callers from probing arbitrary or
// unintended log endpoints. A map is used so membership can be checked in O(1)
// with a single lookup. Each key is a mailcow log source and the boolean value
// is always true, serving purely as a presence marker.
var validLogServices = map[string]bool{
	"postfix":        true, // Postfix SMTP server logs.
	"dovecot":        true, // Dovecot IMAP/POP3 server logs.
	"sogo":           true, // SOGo groupware/webmail logs.
	"watchdog":       true, // mailcow watchdog health-check logs.
	"acme":           true, // ACME/Let's Encrypt certificate management logs.
	"api":            true, // mailcow API access logs.
	"netfilter":      true, // Netfilter/firewall (fail2ban) logs.
	"autodiscover":   true, // Autodiscover/autoconfig client provisioning logs.
	"ratelimited":    true, // Messages rejected or delayed by rate limiting.
	"rspamd-history": true, // Rspamd scanning history entries.
}

// registerLogRoutes wires the HTTP route that serves mailcow log output. It is
// separated into its own function so the log feature owns its route wiring and
// can be registered alongside the other feature route groups. The single route
// exposes read-only access to a named log service and is protected by
// requireAuth so only authenticated operators can read server logs.
func (s *Server) registerLogRoutes(mux *http.ServeMux) {
	// The {service} path segment selects which mailcow log source to fetch; it
	// is validated against validLogServices inside the handler before use.
	mux.HandleFunc("GET /api/logs/{service}", s.requireAuth(s.handleGetLogs))
}

// handleGetLogs serves the most recent entries of a single mailcow log service.
// It exists to translate an authenticated HTTP request into an upstream mailcow
// log fetch while enforcing two safety constraints: the service name must be on
// the allowlist, and the requested entry count must be bounded. The service is
// taken from the {service} path segment and the number of entries from the
// optional ?count= query parameter.
func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	service := r.PathValue("service")
	// Reject any service that is not explicitly allowlisted so that only known
	// mailcow log sources can be queried.
	if !validLogServices[service] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown log service"})
		return
	}
	// Parse and clamp the requested number of entries to a safe range so a
	// caller cannot request an unbounded (or nonsensical) amount of log data.
	count := logCount(r.URL.Query().Get("count"))
	// Stream the raw mailcow log JSON straight through to the client; serveRaw
	// handles context, upstream errors, and writing the response body.
	s.serveRaw(w, r, func(ctx context.Context) (json.RawMessage, error) { return s.mc.Logs(ctx, service, count) })
}

// logCount parses and clamps the ?count= query parameter to the range [1, 1000].
// It exists to give the log endpoint a predictable, bounded page size regardless
// of what the caller supplies. The raw string is the untrusted query value; the
// returned int is always a valid count. When the value is empty or fails to
// parse as an integer, a default of 100 is used; values below 1 are raised to 1
// and values above 1000 are lowered to 1000 to protect the backend from
// excessively large requests.
func logCount(raw string) int {
	count := 100
	// Only override the default when the parameter is both present and a valid
	// integer; malformed input silently falls back to the default.
	if n, err := strconv.Atoi(raw); err == nil && raw != "" {
		count = n
	}
	// Enforce the lower bound so at least one entry is always requested.
	if count < 1 {
		count = 1
	}
	// Enforce the upper bound to cap the amount of log data fetched per request.
	if count > 1000 {
		count = 1000
	}
	return count
}
