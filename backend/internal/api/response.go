package api

import (
	"encoding/json"
	"net/http"

	"github.com/isi1988/Mailfold/backend/internal/mailcow"
)

// errSignInAgain is the shared response body for a pending-login token that
// has expired, been exhausted, or was already redeemed — every second-factor
// verification path (TOTP/recovery, WebAuthn) reports the same message so a
// client can treat them identically.
const errSignInAgain = "sign in again"

// itemsRequest is the shared request body shape for operations that act on a
// list of item identifiers, such as delete and other bulk actions. It models
// the JSON object {"items": [...]}. A single shared type keeps the wire
// contract consistent across every resource that only needs a list of targets.
type itemsRequest struct {
	// Items holds the identifiers (for example mailbox addresses or IDs) the
	// operation should be applied to.
	Items []string `json:"items"`
}

// editRequest is the shared request body shape for edit operations. It models
// the JSON object {"items": [...], "attr": {...}}, pairing the set of targets
// with the attribute changes to apply to each. Keeping it shared means every
// editable resource speaks the same contract.
type editRequest struct {
	// Items holds the identifiers of the objects to modify.
	Items []string `json:"items"`
	// Attr holds the attribute changes to apply. It is an untyped map because
	// mailcow accepts arbitrary attributes and validates them upstream.
	Attr map[string]any `json:"attr"`
}

// writeJSON serializes v as JSON and writes it to the response with the given
// status code and the appropriate Content-Type header. It is the single
// success-path writer used throughout the package so response encoding is
// uniform. The encode error is deliberately ignored because the status line and
// headers have already been committed by the time encoding runs, leaving no
// meaningful way to change the response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError logs the underlying error for operators and returns a JSON error
// body with the given status code to the client. It exists so failure handling
// is consistent: the caller decides the HTTP status (for example 400 for a bad
// body or 502 for an upstream failure) while this method guarantees both a log
// entry and a machine-readable {"error": ...} payload.
func (s *Server) writeError(w http.ResponseWriter, status int, err error) {
	s.logger.Error("request failed", "error", err)
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// decodeJSON decodes the JSON request body into dst. It is a thin wrapper that
// gives every handler one consistent way to parse input and keeps the decoding
// detail (streaming decode of r.Body) in a single place should it ever need to
// change.
func decodeJSON(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

// writeMailcowResults maps a mailcow action-result array onto an HTTP response.
// It returns 200 OK when every entry reports success and 502 Bad Gateway when
// any entry failed, because a partial or total failure at mailcow is an
// upstream problem rather than a client one. The body always carries an "ok"
// flag, a human-readable "message", and the full "results" array so clients can
// both branch on the boolean and inspect per-item detail.
func (s *Server) writeMailcowResults(w http.ResponseWriter, results []mailcow.ActionResult) {
	ok, msg := mailcow.ResultsOK(results)
	status := http.StatusOK
	if !ok {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, map[string]any{"ok": ok, "message": msg, "results": results})
}

// writeUpstream forwards raw JSON received from mailcow directly to the client
// with a 200 OK status. It is used by passthrough GET endpoints where the
// backend does not model the payload and simply relays the upstream bytes. The
// write error is ignored because, as with writeJSON, the response is already
// committed once the body is being streamed.
func writeUpstream(w http.ResponseWriter, raw []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}
