package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/apikey"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
)

// appPwProtocols is the fixed protocol scope of every app-password Mailfold
// mints: IMAP + SMTP only, so a key can never reach POP3, DAV, EAS or Sieve.
var appPwProtocols = []string{"imap_access", "smtp_access"}

// mailboxRe is a deliberately strict address check for the mint endpoint. It
// rejects anything with whitespace, path separators or control characters, which
// (together with url.PathEscape in the mailcow client) closes path-injection.
var mailboxRe = regexp.MustCompile(`^[^\s/@:]+@[^\s/@:]+\.[^\s/@:]+$`)

// apiKeyHandler is a mail handler that has already been authenticated to a
// specific mailbox and its decrypted app-password.
type apiKeyHandler func(w http.ResponseWriter, r *http.Request, mailbox, appPw string)

// registerAPIKeyRoutes mounts the admin key-management endpoints and the
// key-authenticated /api/v1/mail/* surface, but only when the subsystem is
// configured (store + cipher opened in NewServer).
func (s *Server) registerAPIKeyRoutes(mux *http.ServeMux) {
	if s.apikeyStore == nil {
		return
	}

	// Admin-only management (behind the normal admin session).
	mux.HandleFunc("POST /api/apikeys", s.requireAuth(s.handleAPIKeyCreate))
	mux.HandleFunc("GET /api/apikeys", s.requireAuth(s.handleAPIKeyList))
	mux.HandleFunc("DELETE /api/apikeys/{id}", s.requireAuth(s.handleAPIKeyRevoke))

	// Machine surface, authenticated by the API key itself.
	mux.HandleFunc("POST /api/v1/mail/send", s.requireAPIKey(apikey.ScopeSend, s.handleMailSend))
	mux.HandleFunc("GET /api/v1/mail/folders", s.requireAPIKey(apikey.ScopeRead, s.handleMailFolders))
	mux.HandleFunc("GET /api/v1/mail/messages", s.requireAPIKey(apikey.ScopeRead, s.handleMailMessages))
	mux.HandleFunc("GET /api/v1/mail/message", s.requireAPIKey(apikey.ScopeRead, s.handleMailMessage))
	mux.HandleFunc("GET /api/v1/mail/search", s.requireAPIKey(apikey.ScopeRead, s.handleMailSearch))
	mux.HandleFunc("GET /api/v1/mail/attachment", s.requireAPIKey(apikey.ScopeRead, s.handleMailAttachment))
	mux.HandleFunc("POST /api/v1/mail/flag", s.requireAPIKey(apikey.ScopeWrite, s.handleMailFlag))
	mux.HandleFunc("DELETE /api/v1/mail/message", s.requireAPIKey(apikey.ScopeWrite, s.handleMailDelete))
}

func apiKeyUnauthorized(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
}

// rateLimited applies one limiter and, on denial, writes 429 + Retry-After and
// reports true so the caller can stop.
func (s *Server) rateLimited(w http.ResponseWriter, lim *ratelimit.Limiter, key string) bool {
	ok, retry := lim.Allow(key)
	if ok {
		return false
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
	writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
	return true
}

// authenticateAPIKey resolves and validates the presented key, returning the
// record and its decrypted app-password. On any failure it writes the response
// (uniform 401 for unknown/mismatched/revoked/expired — no oracle — or 500) and
// returns ok=false.
func (s *Server) authenticateAPIKey(w http.ResponseWriter, r *http.Request) (*apikey.Record, string, bool) {
	token := bearerToken(r)
	kid, err := apikey.ParseKID(token)
	if err != nil {
		apiKeyUnauthorized(w)
		return nil, "", false
	}
	rec, err := s.apikeyStore.GetByID(kid)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, errors.New("api key lookup failed"))
		return nil, "", false
	}
	if rec == nil || !apikey.TokenMatches(token, rec.TokenSHA256) || !rec.Active(time.Now().UTC()) {
		apiKeyUnauthorized(w)
		return nil, "", false
	}
	appPw, err := s.apikeyCipher.Open(rec.SecretEnc, rec.SecretNonce)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, errors.New("credential decryption failed"))
		return nil, "", false
	}
	return rec, string(appPw), true
}

// requireAPIKey authenticates a request by its API key, enforces rate limits
// (per-IP before auth, per-key after) and the required scope, then invokes next
// with the bound mailbox and its decrypted app-password.
func (s *Server) requireAPIKey(scope string, next apiKeyHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.apikeyStore == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "api keys not configured"})
			return
		}
		// Pre-auth, IP-keyed limit runs before any token parsing or DB lookup so
		// unauthenticated guessing is throttled, not just valid keys.
		if s.rateLimited(w, s.apikeyIPLimit, clientIP(r)) {
			return
		}
		rec, appPw, ok := s.authenticateAPIKey(w, r)
		if !ok {
			return
		}
		if s.rateLimited(w, s.apikeyKeyLimit, rec.ID) {
			return
		}
		if !apikey.HasScope(rec.Scopes, scope) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		_ = s.apikeyStore.TouchLastUsed(rec.ID, time.Now().UTC()) // best-effort
		next(w, r, rec.Mailbox, appPw)
	}
}

type createKeyRequest struct {
	Mailbox          string   `json:"mailbox"`
	Label            string   `json:"label"`
	Scopes           []string `json:"scopes"`
	ExpiresInSeconds int64    `json:"expires_in_seconds"`
}

// handleAPIKeyCreate mints a key: it creates an IMAP+SMTP-scoped mailcow
// app-password, confirms its id, encrypts it, stores the key, and returns the
// full token exactly once. The plaintext token and app-password never appear in
// a log or error.
func (s *Server) handleAPIKeyCreate(w http.ResponseWriter, r *http.Request) {
	var req createKeyRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if !mailboxRe.MatchString(req.Mailbox) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid mailbox address"})
		return
	}
	scopes := req.Scopes
	if len(scopes) == 0 {
		scopes = apikey.DefaultScopes()
	}
	for _, sc := range scopes {
		if !apikey.ValidScope(sc) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown scope: " + sc})
			return
		}
	}

	token, kid, sha, prefix, err := apikey.NewToken()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, errors.New("token generation failed"))
		return
	}
	appPw, err := randomAppPassword()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, errors.New("secret generation failed"))
		return
	}
	appName := "mailfold-apikey:" + kid

	ctx := r.Context()
	// Create the upstream app-password. On any failure return a sentinel error so
	// the request echo (which contains the app-password) can never be logged.
	results, err := s.mc.AddAppPassword(ctx, map[string]any{
		"username":    req.Mailbox,
		"app_name":    appName,
		"app_passwd":  appPw,
		"app_passwd2": appPw,
		"active":      "1",
		"protocols":   appPwProtocols,
	})
	if err != nil {
		s.writeError(w, http.StatusBadGateway, errors.New("mailcow app-password creation failed"))
		return
	}
	if ok, _ := mailcow.ResultsOK(results); !ok {
		s.writeError(w, http.StatusBadGateway, errors.New("mailcow rejected app-password creation"))
		return
	}

	// Confirm the app-password id (needed for hard revoke). If it cannot be
	// uniquely identified, do NOT keep a key we could never revoke upstream:
	// compensating-delete anything we just created and fail.
	appPwID, err := s.recoverAppPwID(ctx, req.Mailbox, appName)
	if err != nil {
		s.deleteAppPwByName(ctx, req.Mailbox, appName)
		s.writeError(w, http.StatusBadGateway, errors.New("could not confirm app-password creation"))
		return
	}

	enc, nonce, err := s.apikeyCipher.Seal([]byte(appPw))
	if err != nil {
		s.deleteAppPwByName(ctx, req.Mailbox, appName)
		s.writeError(w, http.StatusInternalServerError, errors.New("secret encryption failed"))
		return
	}

	now := time.Now().UTC()
	var expires time.Time
	switch {
	case req.ExpiresInSeconds > 0:
		expires = now.Add(time.Duration(req.ExpiresInSeconds) * time.Second)
	case s.cfg.APIKeyDefaultTTL > 0:
		expires = now.Add(s.cfg.APIKeyDefaultTTL)
	}

	rec := apikey.Record{
		ID:          kid,
		TokenSHA256: sha,
		Prefix:      prefix,
		Mailbox:     req.Mailbox,
		Label:       req.Label,
		Scopes:      apikey.JoinScopes(scopes),
		SecretEnc:   enc,
		SecretNonce: nonce,
		MCAppPwID:   appPwID,
		Created:     now,
		Expires:     expires,
	}
	if err := s.apikeyStore.Create(rec); err != nil {
		s.deleteAppPwByName(ctx, req.Mailbox, appName)
		s.writeError(w, http.StatusInternalServerError, errors.New("could not persist api key"))
		return
	}

	resp := map[string]any{
		"id":      kid,
		"token":   token, // shown exactly once
		"mailbox": req.Mailbox,
		"label":   req.Label,
		"scopes":  scopes,
		"prefix":  prefix,
	}
	if !expires.IsZero() {
		resp["expires_at"] = expires
	}
	writeJSON(w, http.StatusCreated, resp)
}

// handleAPIKeyList returns key metadata (never secrets), optionally filtered by
// ?mailbox=.
func (s *Server) handleAPIKeyList(w http.ResponseWriter, r *http.Request) {
	recs, err := s.apikeyStore.List(r.URL.Query().Get("mailbox"))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]map[string]any, 0, len(recs))
	for _, rec := range recs {
		item := map[string]any{
			"id":      rec.ID,
			"mailbox": rec.Mailbox,
			"label":   rec.Label,
			"scopes":  apikey.SplitScopes(rec.Scopes),
			"prefix":  rec.Prefix,
			"created": rec.Created,
			"active":  rec.Active(time.Now().UTC()),
		}
		if !rec.LastUsed.IsZero() {
			item["last_used"] = rec.LastUsed
		}
		if !rec.Expires.IsZero() {
			item["expires_at"] = rec.Expires
		}
		if !rec.Revoked.IsZero() {
			item["revoked_at"] = rec.Revoked
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleAPIKeyRevoke soft-revokes a key and deletes its upstream app-password.
// It is idempotent and returns 404 only when the id was never known.
func (s *Server) handleAPIKeyRevoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, err := s.apikeyStore.GetByID(id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if rec == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if _, err := s.apikeyStore.Revoke(id, time.Now().UTC()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	// Best-effort hard revoke at mailcow so the app-password stops working too.
	if rec.MCAppPwID != "" {
		if _, err := s.mc.DeleteAppPassword(r.Context(), []string{rec.MCAppPwID}); err != nil {
			s.logger.Error("api key revoked locally but mailcow app-password delete failed", "id", id, "error", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// appPwEntry is the subset of a mailcow app-password record we need to identify
// the one we just created.
type appPwEntry struct {
	ID   json.Number `json:"id"`
	Name string      `json:"name"`
}

// recoverAppPwID lists the mailbox's app-passwords and returns the id of the one
// whose name matches appName, requiring exactly one match so revoke can never
// target the wrong password.
func (s *Server) recoverAppPwID(ctx context.Context, mailbox, appName string) (string, error) {
	entries, err := s.listAppPwByName(ctx, mailbox, appName)
	if err != nil {
		return "", err
	}
	if len(entries) != 1 {
		return "", errors.New("app-password id not uniquely identifiable")
	}
	return entries[0].ID.String(), nil
}

func (s *Server) listAppPwByName(ctx context.Context, mailbox, appName string) ([]appPwEntry, error) {
	raw, err := s.mc.AppPasswords(ctx, mailbox)
	if err != nil {
		return nil, err
	}
	var all []appPwEntry
	// mailcow returns {} (not []) for an empty set; tolerate that as zero entries.
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, nil //nolint:nilerr // a non-array body means no entries matched
	}
	var matched []appPwEntry
	for _, e := range all {
		if e.Name == appName {
			matched = append(matched, e)
		}
	}
	return matched, nil
}

// deleteAppPwByName is a best-effort compensating delete used when a mint cannot
// be completed after the upstream app-password was created.
func (s *Server) deleteAppPwByName(ctx context.Context, mailbox, appName string) {
	entries, err := s.listAppPwByName(ctx, mailbox, appName)
	if err != nil || len(entries) == 0 {
		return
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.ID.String())
	}
	if _, err := s.mc.DeleteAppPassword(ctx, ids); err != nil {
		s.logger.Error("compensating app-password delete failed; orphan may remain", "mailbox", mailbox, "app_name", appName, "error", err)
	}
}

// randomAppPassword returns a strong random secret for a mailcow app-password.
func randomAppPassword() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
