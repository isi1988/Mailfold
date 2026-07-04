package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/isi1988/Mailfold/backend/internal/mailcow"
)

// registerWebmailRuleRoutes wires a simple "if <field> contains <value>, move
// to <folder>" rule builder for the currently authenticated mailbox, on top
// of mailcow's existing Sieve filter primitives (the same ones the admin-side
// MailboxDrawer "Sieve filters" section already manages via /api/filters).
// Every operation here is scoped to the caller's own mailbox: unlike
// /api/filters (an admin-only passthrough over every mailbox's filters), a
// webmail user can only ever see or touch filters whose username is their own.
func (s *Server) registerWebmailRuleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/webmail/rules", s.requireWebmail(s.handleWebmailListRules))
	mux.HandleFunc("POST /api/webmail/rules", s.requireWebmail(s.handleWebmailCreateRule))
	mux.HandleFunc("PUT /api/webmail/rules", s.requireWebmail(s.handleWebmailEditRule))
	mux.HandleFunc("DELETE /api/webmail/rules", s.requireWebmail(s.handleWebmailDeleteRule))
}

// webmailRuleFieldHeaders maps the field choices the frontend offers to the
// literal mail header Sieve tests against.
var webmailRuleFieldHeaders = map[string]string{
	"from":    "from",
	"to":      "to",
	"subject": "subject",
}

// mailcowFilter mirrors the fields of mailcow's GET /api/v1/get/filters/all
// entries that this feature needs; the upstream payload carries a few more
// (e.g. active as 0/1), which json.Unmarshal simply ignores.
type mailcowFilter struct {
	ID         int    `json:"id"`
	Username   string `json:"username"`
	ScriptData string `json:"script_data"`
	ScriptDesc string `json:"script_desc"`
	Active     int    `json:"active"`
}

// ruleMeta is the structured rule Mailfold actually cares about, round-tripped
// through a Sieve comment (see sieveRuleScript) rather than parsed back out of
// the executable Sieve syntax — far more robust than re-lexing our own
// generated script, and invisible to mailcow/dovecot, which only run the
// commands below the comment.
type ruleMeta struct {
	Field  string `json:"field"`
	Value  string `json:"value"`
	Folder string `json:"folder"`
}

const ruleMetaPrefix = "# mailfold-rule: "

// sieveRuleScript builds the Sieve script for one condition/action rule,
// prefixed with a machine-readable comment carrying the exact rule metadata
// so handleWebmailListRules can reconstruct it without parsing Sieve syntax.
func sieveRuleScript(field, value, folder string) (string, error) {
	meta, err := json.Marshal(ruleMeta{Field: field, Value: value, Folder: folder})
	if err != nil {
		return "", err
	}
	header := webmailRuleFieldHeaders[field]
	return fmt.Sprintf("%s%s\r\nrequire [\"fileinto\"];\r\nif header :contains %s %s {\r\n    fileinto %s;\r\n}\r\n",
		ruleMetaPrefix, meta, sieveQuote(header), sieveQuote(value), sieveQuote(folder)), nil
}

// sieveQuote renders s as a Sieve quoted-string literal (RFC 5228 §2.4.2):
// backslash and double-quote are the only characters requiring escape.
func sieveQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// parseRuleMeta extracts the ruleMeta comment written by sieveRuleScript, if
// present. A script without it (e.g. hand-written by an admin in the Sieve
// textarea) simply isn't a rule the webmail rule builder understands, and is
// omitted from the list rather than shown as a broken entry.
func parseRuleMeta(scriptData string) (ruleMeta, bool) {
	firstLine, _, _ := strings.Cut(scriptData, "\n")
	firstLine = strings.TrimSuffix(firstLine, "\r")
	if !strings.HasPrefix(firstLine, ruleMetaPrefix) {
		return ruleMeta{}, false
	}
	var m ruleMeta
	if err := json.Unmarshal([]byte(strings.TrimPrefix(firstLine, ruleMetaPrefix)), &m); err != nil {
		return ruleMeta{}, false
	}
	return m, true
}

// webmailRule is the wire shape the webmail rule-builder UI speaks — a plain
// condition/action pair, not raw Sieve.
type webmailRule struct {
	ID           int    `json:"id"`
	Description  string `json:"description"`
	Field        string `json:"field"`
	Value        string `json:"value"`
	TargetFolder string `json:"target_folder"`
	Active       bool   `json:"active"`
}

// ownFilters fetches every mailcow filter and returns only those belonging to
// email that were created by the rule builder (i.e. carry a ruleMeta
// comment), alongside the full raw list (needed by edit/delete to verify
// ownership of a specific id without a second round trip).
func (s *Server) ownFilters(ctx context.Context, email string) ([]mailcowFilter, error) {
	raw, err := s.mc.Filters(ctx)
	if err != nil {
		return nil, err
	}
	var all []mailcowFilter
	// mailcow returns {} rather than [] when there are no filters at all.
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" {
		return nil, nil
	}
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, err
	}
	out := make([]mailcowFilter, 0, len(all))
	for _, f := range all {
		if f.Username == email {
			out = append(out, f)
		}
	}
	return out, nil
}

func (s *Server) handleWebmailListRules(w http.ResponseWriter, r *http.Request) {
	filters, err := s.ownFilters(r.Context(), webmailCreds(r).Email)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	rules := make([]webmailRule, 0, len(filters))
	for _, f := range filters {
		meta, ok := parseRuleMeta(f.ScriptData)
		if !ok {
			continue
		}
		rules = append(rules, webmailRule{
			ID: f.ID, Description: f.ScriptDesc, Field: meta.Field, Value: meta.Value,
			TargetFolder: meta.Folder, Active: f.Active != 0,
		})
	}
	writeJSON(w, http.StatusOK, rules)
}

type webmailRuleRequest struct {
	Description  string `json:"description"`
	Field        string `json:"field"`
	Value        string `json:"value"`
	TargetFolder string `json:"target_folder"`
	Active       bool   `json:"active"`
}

func (s *Server) validateRuleRequest(req webmailRuleRequest) error {
	if _, ok := webmailRuleFieldHeaders[req.Field]; !ok {
		return fmt.Errorf("field must be one of from, to, subject")
	}
	if strings.TrimSpace(req.Value) == "" {
		return fmt.Errorf("value must not be empty")
	}
	if strings.TrimSpace(req.TargetFolder) == "" {
		return fmt.Errorf("target_folder must not be empty")
	}
	return nil
}

func (s *Server) handleWebmailCreateRule(w http.ResponseWriter, r *http.Request) {
	var req webmailRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.validateRuleRequest(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	script, err := sieveRuleScript(req.Field, req.Value, req.TargetFolder)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	active := "0"
	if req.Active {
		active = "1"
	}
	s.runAction(w, r, func(ctx context.Context) ([]mailcow.ActionResult, error) {
		return s.mc.AddFilter(ctx, map[string]any{
			"username":    webmailCreds(r).Email,
			"filter_type": "postfilter",
			"script_desc": req.Description,
			"script_data": script,
			"active":      active,
		})
	})
}

// requireOwnedFilter verifies id belongs to email (was created by the rule
// builder for their own mailbox) before an edit/delete proceeds, so a webmail
// user cannot guess another mailbox's filter id to read or mutate it.
func (s *Server) requireOwnedFilter(w http.ResponseWriter, r *http.Request, id string) bool {
	filters, err := s.ownFilters(r.Context(), webmailCreds(r).Email)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return false
	}
	for _, f := range filters {
		if strconv.Itoa(f.ID) == id {
			if _, ok := parseRuleMeta(f.ScriptData); ok {
				return true
			}
			break
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "rule not found"})
	return false
}

func (s *Server) handleWebmailEditRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
		webmailRuleRequest
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.validateRuleRequest(req.webmailRuleRequest); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !s.requireOwnedFilter(w, r, req.ID) {
		return
	}
	script, err := sieveRuleScript(req.Field, req.Value, req.TargetFolder)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	active := "0"
	if req.Active {
		active = "1"
	}
	s.runAction(w, r, func(ctx context.Context) ([]mailcow.ActionResult, error) {
		return s.mc.EditFilter(ctx, []string{req.ID}, map[string]any{
			"script_desc": req.Description,
			"script_data": script,
			"active":      active,
		})
	})
}

func (s *Server) handleWebmailDeleteRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if !s.requireOwnedFilter(w, r, req.ID) {
		return
	}
	s.runAction(w, r, func(ctx context.Context) ([]mailcow.ActionResult, error) {
		return s.mc.DeleteFilter(ctx, []string{req.ID})
	})
}
