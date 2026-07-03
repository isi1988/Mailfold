package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/webmail"
)

// webmailEventInterval is how often the events stream polls the INBOX for new
// mail. IMAP is polled per connected client, so this trades notification latency
// against server load. It is a var so tests can shorten it.
var webmailEventInterval = 20 * time.Second

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
	mux.HandleFunc("POST /api/webmail/external", s.requireWebmail(s.handleWebmailExternal))
	// The events stream authenticates from a query parameter, not a bearer
	// header, because the browser EventSource API cannot set request headers.
	mux.HandleFunc("GET /api/webmail/events", s.handleWebmailEvents)
}

// externalSyncRequest describes an external IMAP account the user wants to pull
// into their own mailbox.
type externalSyncRequest struct {
	Host       string `json:"host"`
	Port       string `json:"port"`
	User       string `json:"user"`
	Password   string `json:"password"`
	Encryption string `json:"encryption"` // SSL | TLS | PLAIN
	Interval   int    `json:"interval"`   // minutes between syncs
}

// handleWebmailExternal connects an external mailbox by creating a mailcow sync
// job that imports it into the *logged-in* mailbox — a webmail user can only
// pull mail into their own account (the target is forced to their address), so
// this endpoint cannot be used to write into someone else's mailbox.
func (s *Server) handleWebmailExternal(w http.ResponseWriter, r *http.Request) {
	cred := webmailCreds(r)
	var req externalSyncRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Host == "" || req.User == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "host and user are required"})
		return
	}
	port := req.Port
	if port == "" {
		port = "993"
	}
	enc := req.Encryption
	if enc != "TLS" && enc != "PLAIN" {
		enc = "SSL"
	}
	interval := req.Interval
	if interval <= 0 {
		interval = 15
	}
	attr := map[string]any{
		"username":          cred.Email, // import INTO the logged-in mailbox only
		"host1":             req.Host,
		"port1":             port,
		"user1":             req.User,
		"password1":         req.Password,
		"enc1":              enc,
		"mins_interval":     interval,
		"active":            "1",
		"delete2duplicates": "1",
	}
	results, err := s.mc.AddSyncJob(r.Context(), attr)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	if ok, _ := mailcow.ResultsOK(results); !ok {
		s.writeError(w, http.StatusBadGateway, errors.New("mailcow rejected the sync job"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "connected"})
}

// handleWebmailEvents is a Server-Sent Events stream that notifies the client
// when new mail arrives in the INBOX. It polls IMAP every webmailEventInterval
// for messages above the highest UID seen at connect time and emits a "mail"
// event with the new headers. Comment lines act as keepalives. The stream ends
// when the client disconnects.
func (s *Server) handleWebmailEvents(w http.ResponseWriter, r *http.Request) {
	cred, ok := s.webmailSessions.Get(r.URL.Query().Get("token"))
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, errors.New("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering (nginx)
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, ": connected\n\n")
	flusher.Flush()

	ctx := r.Context()
	// Establish the baseline so only genuinely new mail is reported. A transient
	// IMAP error here is non-fatal; the stream stays open and retries on the tick.
	_, sinceUID, err := s.webmail.CheckSince(cred.Email, cred.Password, 0)
	if err != nil {
		s.logger.Warn("webmail events baseline failed", "email", cred.Email, "error", err)
	}

	ticker := time.NewTicker(webmailEventInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			msgs, newUID, err := s.webmail.CheckSince(cred.Email, cred.Password, sinceUID)
			if err != nil {
				_, _ = io.WriteString(w, ": ping\n\n") // keepalive; skip this tick
				flusher.Flush()
				continue
			}
			sinceUID = newUID
			if len(msgs) == 0 {
				_, _ = io.WriteString(w, ": ping\n\n")
				flusher.Flush()
				continue
			}
			payload, _ := json.Marshal(map[string]any{"count": len(msgs), "messages": msgs})
			if _, err := io.WriteString(w, "event: mail\ndata: "+string(payload)+"\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
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
		// The client always sees a uniform 401 (no user-enumeration or
		// password-vs-connection oracle), but log the underlying cause — an
		// actual auth failure looks very different from an IMAP/TLS/connection
		// error, and swallowing it silently makes webmail outages hard to
		// diagnose. The password is never logged.
		s.logger.Warn("webmail login verification failed", "email", req.Email, "error", err)
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
	// Save a copy to the Sent folder. Best-effort: the mail has already been
	// submitted, so a failed copy is logged, not surfaced as a send failure.
	if err := s.webmail.SaveToSent(cred.Email, cred.Password, &msg); err != nil {
		s.logger.Warn("message sent but not saved to Sent folder", "email", cred.Email, "error", err)
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
