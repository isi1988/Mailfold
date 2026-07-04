package api

import (
	"errors"
	"net/http"
	"time"
)

// deviceLoginRequest is the request body for POST /api/auth/device-login.
type deviceLoginRequest struct {
	Key string `json:"key"`
}

// registerDeviceLoginRoutes wires signing into webmail with a personal
// Mailfold API key instead of the mailbox password — useful for a new device
// that only holds a key (a mail client, a script, a phone being set up) and
// should not need to know or re-enter the mailbox password. Any active key
// works regardless of its declared mail:* scopes: the app-password behind
// every key is always fully IMAP+SMTP capable, so gating this by scope would
// not add real security, only friction.
func (s *Server) registerDeviceLoginRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/device-login", s.handleDeviceLogin)
}

func (s *Server) handleDeviceLogin(w http.ResponseWriter, r *http.Request) {
	if s.apikeyStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "api keys are not configured"})
		return
	}
	if s.rateLimited(w, s.apikeyIPLimit, clientIP(r)) {
		return
	}
	var req deviceLoginRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	rec, appPw, err := s.resolveAPIKey(req.Key)
	if err != nil {
		if errors.Is(err, errAPIKeyInvalid) {
			apiKeyUnauthorized(w)
		} else {
			s.writeError(w, http.StatusInternalServerError, err)
		}
		return
	}
	_ = s.apikeyStore.TouchLastUsed(rec.ID, time.Now().UTC()) // best-effort
	token, _, err := s.webmailSessions.Create(rec.Mailbox, appPw)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token, "email": rec.Mailbox})
}
