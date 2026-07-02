package api

import (
	"encoding/json"
	"net/http"
)

// handleHealth is a simple liveness probe.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "mailfold"})
}

// handleDomains lists mail domains from mailcow.
func (s *Server) handleDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := s.mc.Domains(r.Context())
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, domains)
}

// handleMailboxes lists mailboxes from mailcow.
func (s *Server) handleMailboxes(w http.ResponseWriter, r *http.Request) {
	mailboxes, err := s.mc.Mailboxes(r.Context())
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, mailboxes)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) writeError(w http.ResponseWriter, status int, err error) {
	s.logger.Error("request failed", "error", err)
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
