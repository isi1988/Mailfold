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
	mux.HandleFunc("POST /api/webmail/flag", s.requireWebmail(s.handleWebmailFlag))
	mux.HandleFunc("POST /api/webmail/move", s.requireWebmail(s.handleWebmailMove))
	mux.HandleFunc("POST /api/webmail/delete", s.requireWebmail(s.handleWebmailDelete))
	mux.HandleFunc("POST /api/webmail/folders", s.requireWebmail(s.handleWebmailCreateFolder))
	mux.HandleFunc("GET /api/webmail/search", s.requireWebmail(s.handleWebmailSearch))
	mux.HandleFunc("GET /api/webmail/attachment", s.requireWebmail(s.handleWebmailAttachment))
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

type webmailFlagRequest struct {
	Folder string `json:"folder"`
	UID    uint32 `json:"uid"`
	Flag   string `json:"flag"`
	Set    bool   `json:"set"`
}

// handleWebmailFlag adds or removes a system flag on a message.
func (s *Server) handleWebmailFlag(w http.ResponseWriter, r *http.Request) {
	cred := webmailCreds(r)
	var req webmailFlagRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.webmail.SetFlag(cred.Email, cred.Password, orInbox(req.Folder), req.UID, req.Flag, req.Set); err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type webmailMoveRequest struct {
	Folder string `json:"folder"`
	UID    uint32 `json:"uid"`
	Target string `json:"target"`
}

// handleWebmailMove moves a message to another folder.
func (s *Server) handleWebmailMove(w http.ResponseWriter, r *http.Request) {
	cred := webmailCreds(r)
	var req webmailMoveRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.webmail.Move(cred.Email, cred.Password, orInbox(req.Folder), req.UID, req.Target); err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type webmailUIDRequest struct {
	Folder string `json:"folder"`
	UID    uint32 `json:"uid"`
}

// handleWebmailDelete permanently deletes a message.
func (s *Server) handleWebmailDelete(w http.ResponseWriter, r *http.Request) {
	cred := webmailCreds(r)
	var req webmailUIDRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.webmail.Delete(cred.Email, cred.Password, orInbox(req.Folder), req.UID); err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type webmailFolderRequest struct {
	Name string `json:"name"`
}

// handleWebmailCreateFolder creates a new mail folder.
func (s *Server) handleWebmailCreateFolder(w http.ResponseWriter, r *http.Request) {
	cred := webmailCreds(r)
	var req webmailFolderRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "folder name is required"})
		return
	}
	if err := s.webmail.CreateFolder(cred.Email, cred.Password, req.Name); err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "created"})
}

// handleWebmailSearch returns message headers matching a free-text query.
func (s *Server) handleWebmailSearch(w http.ResponseWriter, r *http.Request) {
	cred := webmailCreds(r)
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query parameter q is required"})
		return
	}
	msgs, err := s.webmail.Search(cred.Email, cred.Password, folderParam(r), q)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, msgs)
}

// handleWebmailAttachment streams a message attachment as a download.
func (s *Server) handleWebmailAttachment(w http.ResponseWriter, r *http.Request) {
	cred := webmailCreds(r)
	uid, err := strconv.ParseUint(r.URL.Query().Get("uid"), 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid uid"})
		return
	}
	index, _ := strconv.Atoi(r.URL.Query().Get("index"))
	filename, contentType, data, err := s.webmail.Attachment(cred.Email, cred.Password, folderParam(r), uint32(uid), index)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	if filename != "" {
		w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	}
	_, _ = w.Write(data)
}

// folderParam returns the ?folder= query value, defaulting to INBOX.
func folderParam(r *http.Request) string {
	return orInbox(r.URL.Query().Get("folder"))
}

// orInbox returns folder, or "INBOX" when folder is empty.
func orInbox(folder string) string {
	if folder != "" {
		return folder
	}
	return "INBOX"
}
