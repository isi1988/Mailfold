package api

import (
	"net/http"
	"time"
)

// registerWebmailSignatureRoutes wires a webmail user's own email signature —
// distinct from internal/account's admin profile settings, this is scoped to
// the authenticated mailbox itself so each linked account can carry its own.
func (s *Server) registerWebmailSignatureRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/webmail/signature", s.requireWebmail(s.handleWebmailGetSignature))
	mux.HandleFunc("PUT /api/webmail/signature", s.requireWebmail(s.handleWebmailSetSignature))
}

// requireWebmailUserStore reports 501 when the webmail-user store is
// unavailable (no MAILFOLD_DB_PATH configured), so the frontend can hide the
// feature rather than offer a control that would fail.
func (s *Server) requireWebmailUserStore(w http.ResponseWriter) bool {
	if s.webmailUsers == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "set MAILFOLD_DB_PATH to enable webmail signatures"})
		return false
	}
	return true
}

func (s *Server) handleWebmailGetSignature(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebmailUserStore(w) {
		return
	}
	acct, err := s.webmailUsers.GetAccount(webmailCreds(r).Email)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"signature": acct.Signature})
}

type webmailSignatureRequest struct {
	Signature string `json:"signature"`
}

func (s *Server) handleWebmailSetSignature(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebmailUserStore(w) {
		return
	}
	var req webmailSignatureRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.webmailUsers.SetSignature(webmailCreds(r).Email, req.Signature, time.Now()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
