package api

import (
	"net/http"
	"strconv"

	"github.com/isi1988/Mailfold/backend/internal/webmail"
)

// maxMailBodyBytes caps a single send's body so an authorized key cannot be used
// to push arbitrarily large messages (belt-and-braces alongside MaxBodyBytes).
const maxMailBodyBytes = 1 << 20

// handleMailSend submits a message as the bound mailbox. From is forced to that
// mailbox by webmail.Send (no spoofing); recipient count and body size are capped
// so a key cannot become an unbounded bulk mailer.
func (s *Server) handleMailSend(w http.ResponseWriter, r *http.Request, mailbox, appPw string) {
	var msg webmail.OutgoingMessage
	if err := decodeJSON(r, &msg); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if n := len(msg.To) + len(msg.Cc) + len(msg.Bcc); n > s.cfg.APIKeyMaxRecipients {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "too many recipients"})
		return
	}
	if len(msg.Text)+len(msg.HTML) > maxMailBodyBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "message body too large"})
		return
	}
	if err := s.webmail.Send(mailbox, appPw, &msg); err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// handleMailFolders lists the mailbox's IMAP folders.
func (s *Server) handleMailFolders(w http.ResponseWriter, r *http.Request, mailbox, appPw string) {
	folders, err := s.webmail.Folders(mailbox, appPw)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, folders)
}

// handleMailMessages lists message headers newest-first (the collect endpoint).
func (s *Server) handleMailMessages(w http.ResponseWriter, r *http.Request, mailbox, appPw string) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	msgs, err := s.webmail.Messages(mailbox, appPw, folderParam(r), limit)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, msgs)
}

// handleMailMessage fetches one full message by uid.
func (s *Server) handleMailMessage(w http.ResponseWriter, r *http.Request, mailbox, appPw string) {
	uid, err := strconv.ParseUint(r.URL.Query().Get("uid"), 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid uid"})
		return
	}
	msg, err := s.webmail.Message(mailbox, appPw, folderParam(r), uint32(uid))
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, msg)
}

// handleMailSearch runs an IMAP text search.
func (s *Server) handleMailSearch(w http.ResponseWriter, r *http.Request, mailbox, appPw string) {
	msgs, err := s.webmail.Search(mailbox, appPw, folderParam(r), r.URL.Query().Get("q"))
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, msgs)
}

// handleMailAttachment streams one attachment's bytes.
func (s *Server) handleMailAttachment(w http.ResponseWriter, r *http.Request, mailbox, appPw string) {
	uid, err := strconv.ParseUint(r.URL.Query().Get("uid"), 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid uid"})
		return
	}
	index, _ := strconv.Atoi(r.URL.Query().Get("index"))
	filename, contentType, data, err := s.webmail.Attachment(mailbox, appPw, folderParam(r), uint32(uid), index)
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

// handleMailFlag marks a collected message (defaulting to \Seen) so the next
// poll can filter it out.
func (s *Server) handleMailFlag(w http.ResponseWriter, r *http.Request, mailbox, appPw string) {
	var req webmailFlagRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	flag := req.Flag
	if flag == "" {
		flag = "\\Seen"
	}
	if err := s.webmail.SetFlag(mailbox, appPw, orInbox(req.Folder), req.UID, flag, req.Set); err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleMailDelete permanently deletes a collected message.
func (s *Server) handleMailDelete(w http.ResponseWriter, r *http.Request, mailbox, appPw string) {
	var req webmailUIDRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.webmail.Delete(mailbox, appPw, orInbox(req.Folder), req.UID); err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
