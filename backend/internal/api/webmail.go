package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/isi1988/Mailfold/backend/internal/webmail"
)

// webmailCtxKey is the context key holding the authenticated webmail credentials.
const webmailCtxKey ctxKey = "webmail"

// registerWebmailRoutes wires the end-user webmail endpoints. These are distinct
// from the admin API: a caller authenticates with their own mailbox credentials
// and the handlers act on that user's mail over IMAP/SMTP.
func (s *Server) registerWebmailRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/webmail/login", s.handleWebmailLogin)
	mux.HandleFunc("POST /api/webmail/logout", s.requireWebmail(s.handleWebmailLogout))
	mux.HandleFunc("GET /api/webmail/folders", s.requireWebmail(s.handleWebmailFolders))
	mux.HandleFunc("GET /api/webmail/messages", s.requireWebmail(s.handleWebmailMessages))
	mux.HandleFunc("GET /api/webmail/message", s.requireWebmail(s.handleWebmailMessage))
	mux.HandleFunc("POST /api/webmail/send", s.requireWebmail(s.handleWebmailSend))
}

// requireWebmail authenticates a webmail request from its bearer token and
// attaches the user's credentials to the request context.
func (s *Server) requireWebmail(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cred, ok := s.webmailSessions.Get(bearerToken(r))
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), webmailCtxKey, cred)))
	}
}

func webmailCreds(r *http.Request) *webmail.Credentials {
	cred, _ := r.Context().Value(webmailCtxKey).(*webmail.Credentials)
	return cred
}

type webmailLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// handleWebmailLogin verifies mailbox credentials against IMAP and, on success,
// issues a session token the client uses for subsequent webmail calls.
func (s *Server) handleWebmailLogin(w http.ResponseWriter, r *http.Request) {
	if !s.webmail.Configured() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "webmail is not configured"})
		return
	}
	// Reuse the login rate limiter to throttle mailbox password guessing.
	if allowed, retry := s.loginLimiter.Allow(clientIP(r)); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many login attempts, slow down"})
		return
	}
	var req webmailLoginRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.webmail.Verify(req.Email, req.Password); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	token, exp, err := s.webmailSessions.Create(req.Email, req.Password)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "email": req.Email, "expires_at": exp})
}

func (s *Server) handleWebmailLogout(w http.ResponseWriter, r *http.Request) {
	s.webmailSessions.Delete(bearerToken(r))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleWebmailFolders(w http.ResponseWriter, r *http.Request) {
	cred := webmailCreds(r)
	folders, err := s.webmail.Folders(cred.Email, cred.Password)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, folders)
}

func (s *Server) handleWebmailMessages(w http.ResponseWriter, r *http.Request) {
	cred := webmailCreds(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	msgs, err := s.webmail.Messages(cred.Email, cred.Password, folderParam(r), limit)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, msgs)
}

func (s *Server) handleWebmailMessage(w http.ResponseWriter, r *http.Request) {
	cred := webmailCreds(r)
	uid, err := strconv.ParseUint(r.URL.Query().Get("uid"), 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid uid"})
		return
	}
	msg, err := s.webmail.Message(cred.Email, cred.Password, folderParam(r), uint32(uid))
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, msg)
}

// handleWebmailSend composes and submits a message on behalf of the user.
func (s *Server) handleWebmailSend(w http.ResponseWriter, r *http.Request) {
	cred := webmailCreds(r)
	var msg webmail.OutgoingMessage
	if err := decodeJSON(r, &msg); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.webmail.Send(cred.Email, cred.Password, &msg); err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// folderParam returns the ?folder= query value, defaulting to INBOX.
func folderParam(r *http.Request) string {
	if f := r.URL.Query().Get("folder"); f != "" {
		return f
	}
	return "INBOX"
}
