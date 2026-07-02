package mailcow

import (
	"encoding/json"
	"strconv"
	"strings"
)

// FlexInt64 is an int64 that also decodes from a JSON string. mailcow is
// inconsistent about numeric fields — byte counts such as a domain's total and
// quota are returned as quoted strings once they are non-zero — so fields that
// can arrive either way use this type. It marshals back out as a plain number.
type FlexInt64 int64

// UnmarshalJSON accepts a JSON number, a quoted number, null, or an empty string.
func (f *FlexInt64) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*f = FlexInt64(n)
	return nil
}

// ActionResult is one element of the JSON array that mailcow returns from its
// mutating endpoints (add/edit/delete/flush). mailcow reports the outcome of an
// operation as a list of these entries rather than a single status code, so the
// caller must inspect every element to know whether the whole operation
// succeeded. It exists to give the rest of the backend a typed view over that
// otherwise loosely-structured response.
type ActionResult struct {
	// Type is the severity/category of the entry. mailcow uses values such as
	// "success", "info", "warning", "danger", or "error". Only "success" and
	// "info" are treated as non-failures elsewhere in the package.
	Type string `json:"type"`
	// Msg is the human-readable message payload. It is kept as raw JSON because
	// mailcow is inconsistent about its shape (sometimes a plain string,
	// sometimes an array of strings or a nested structure), and we want to pass
	// it through to the frontend without lossy re-encoding.
	Msg json.RawMessage `json:"msg,omitempty"`
	// Log is the optional raw log/context that mailcow attaches to an entry. It
	// is retained verbatim for diagnostics and, like Msg, kept as raw JSON to
	// avoid assuming a fixed schema.
	Log json.RawMessage `json:"log,omitempty"`
}

// EditRequest is the canonical request body shape that mailcow expects for its
// edit endpoints: {"items": [...], "attr": {...}}. It exists so every EditXxx
// method can build the same well-formed payload instead of hand-assembling maps,
// which keeps the wire format consistent across all resources.
type EditRequest struct {
	// Items is the list of resource identifiers to modify (domain names,
	// usernames, alias ids as strings, and so on). mailcow applies Attr to each
	// of these items.
	Items []string `json:"items"`
	// Attr is the set of attributes to change on every listed item. It is
	// deliberately untyped (any) because each resource kind accepts a different
	// attribute set, and the concrete shape is validated by mailcow itself.
	Attr any `json:"attr"`
}

// ResultsOK reduces a slice of ActionResult entries into a single pass/fail
// verdict plus a combined human-readable message. It exists so callers do not
// have to re-implement the "is this whole operation a success?" logic, which is
// subtle because mailcow spreads the outcome across many entries and treats both
// "success" and "info" as acceptable. The returned string aggregates all the
// per-entry messages so the reason for a failure can be surfaced to the user.
func ResultsOK(results []ActionResult) (bool, string) {
	// Start optimistically; any single non-success/non-info entry flips this.
	ok := true
	// Pre-size the message buffer to the number of entries to avoid re-growth.
	msgs := make([]string, 0, len(results))
	for _, r := range results {
		// mailcow considers only "success" and "info" as good outcomes;
		// anything else (warning/danger/error) means the operation failed.
		if r.Type != "success" && r.Type != "info" {
			ok = false
		}
		// Collect every non-empty message so the caller can report exactly what
		// mailcow said, regardless of whether the entry was a success.
		if len(r.Msg) > 0 {
			msgs = append(msgs, string(r.Msg))
		}
	}
	// Join with "; " so multiple messages read as a single sentence-like string.
	return ok, strings.Join(msgs, "; ")
}
