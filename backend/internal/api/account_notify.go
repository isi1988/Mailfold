package api

import (
	"net/http"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/webmail"
)

// registerNotifySenderRoutes wires the system-notification sender: the mailbox
// (and its password) Mailfold authenticates as when it needs to email the admin
// directly, currently only used for the forgot-password flow. It is
// admin-configurable rather than a fixed environment variable so the operator
// can pick any real mailbox already hosted on their mailcow instance.
func (s *Server) registerNotifySenderRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/account/notify-sender", s.requireAuth(s.handleNotifySenderGet))
	mux.HandleFunc("PUT /api/account/notify-sender", s.requireAuth(s.handleNotifySenderPut))
	mux.HandleFunc("POST /api/account/notify-sender/test", s.requireAuth(s.handleNotifySenderTest))
}

// notifySenderResponse never includes the stored password; Configured tells the
// frontend whether a working sender is set up without exposing the secret.
type notifySenderResponse struct {
	Mailbox    string `json:"mailbox"`
	Configured bool   `json:"configured"`
}

func (s *Server) handleNotifySenderGet(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminCipher(w) {
		return
	}
	acct, err := s.adminStore.GetAccount(s.cfg.AdminUser)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, notifySenderResponse{
		Mailbox:    acct.NotifyMailbox,
		Configured: acct.NotifyMailbox != "" && len(acct.NotifyPasswordEnc) > 0,
	})
}

type notifySenderRequest struct {
	CurrentPassword string `json:"current_password"`
	Mailbox         string `json:"mailbox"`
	Password        string `json:"password"`
}

// handleNotifySenderPut sets or clears the notification sender. Setting one
// requires the current admin password (this mailbox's credentials will later be
// used to send email on the admin's behalf) and is verified against the real
// IMAP server before being stored, so a typo'd password is caught immediately
// rather than silently breaking the forgot-password flow later. Submitting an
// empty mailbox clears the configuration.
func (s *Server) handleNotifySenderPut(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminCipher(w) {
		return
	}
	var req notifySenderRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if !s.auth.CheckPassword(s.cfg.AdminUser, req.CurrentPassword) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "current password is incorrect"})
		return
	}

	if req.Mailbox == "" {
		if err := s.adminStore.SetNotifySender(s.cfg.AdminUser, "", nil, nil, time.Now()); err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	if !s.webmail.Configured() {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "set MAILFOLD_IMAP_ADDR and MAILFOLD_SMTP_ADDR to enable the notification sender"})
		return
	}
	if err := s.webmail.Verify(req.Mailbox, req.Password); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "could not sign in to that mailbox with the given password"})
		return
	}
	enc, nonce, err := s.adminCipher.Seal([]byte(req.Password))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.adminStore.SetNotifySender(s.cfg.AdminUser, req.Mailbox, enc, nonce, time.Now()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleNotifySenderTest sends a real email to the admin's profile address so
// they can confirm the configured sender actually works end to end.
func (s *Server) handleNotifySenderTest(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminCipher(w) {
		return
	}
	acct, err := s.adminStore.GetAccount(s.cfg.AdminUser)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if acct.NotifyMailbox == "" || len(acct.NotifyPasswordEnc) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "configure a notification sender first"})
		return
	}
	if acct.Email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "set a profile email to receive the test message"})
		return
	}
	pw, err := s.adminCipher.Open(acct.NotifyPasswordEnc, acct.NotifyPasswordNonce)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	msg := &webmail.OutgoingMessage{
		To:      []string{acct.Email},
		Subject: "Mailfold test notification",
		Text:    "This is a test message from Mailfold's notification sender. If you received this, the configured mailbox works.",
	}
	if err := s.webmail.Send(acct.NotifyMailbox, string(pw), msg); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to send: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}
