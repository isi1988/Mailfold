package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/sharedmailbox"
)

// registerSharedMailboxAdminRoutes wires the super-admin's shared/team
// mailbox management: creating and deleting shared mailboxes, and granting
// or revoking a webmail user's access to one. The webmail-facing surface —
// discovering which shared mailboxes a member has access to, activating one
// as a session, and per-message assignment/notes — is registered separately
// in webmail_shared.go.
func (s *Server) registerSharedMailboxAdminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/shared-mailboxes", s.requireAuth(s.handleListSharedMailboxes))
	mux.HandleFunc("POST /api/shared-mailboxes", s.requireAuth(s.handleCreateSharedMailbox))
	mux.HandleFunc("DELETE /api/shared-mailboxes", s.requireAuth(s.handleDeleteSharedMailbox))
	mux.HandleFunc("POST /api/shared-mailboxes/members", s.requireAuth(s.handleAddSharedMailboxMember))
	mux.HandleFunc("DELETE /api/shared-mailboxes/members", s.requireAuth(s.handleRemoveSharedMailboxMember))
}

// requireSharedMailboxStore reports 501 when shared mailboxes are
// unavailable (no database, or no MAILFOLD_ADMIN_ENC_KEY to encrypt the
// delegated app-password with) — the same two-part gate as SSO's cached
// mailbox credential, which stores the identical kind of secret.
func (s *Server) requireSharedMailboxStore(w http.ResponseWriter) bool {
	if s.sharedMailboxes == nil || s.adminCipher == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "set MAILFOLD_DB_PATH and MAILFOLD_ADMIN_ENC_KEY to enable shared mailboxes"})
		return false
	}
	return true
}

// sharedMailboxView is the wire shape for a shared mailbox — the app-password
// is never returned, only the mailbox identity, its members, and who set it up.
type sharedMailboxView struct {
	ID          int64     `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Members     []string  `json:"members"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *Server) toSharedMailboxView(m sharedmailbox.Mailbox) (sharedMailboxView, error) {
	members, err := s.sharedMailboxes.ListMembers(m.ID)
	if err != nil {
		return sharedMailboxView{}, err
	}
	return sharedMailboxView{
		ID: m.ID, Email: m.Email, DisplayName: m.DisplayName,
		Members: members, CreatedBy: m.CreatedBy, CreatedAt: m.CreatedAt,
	}, nil
}

func (s *Server) handleListSharedMailboxes(w http.ResponseWriter, r *http.Request) {
	if !s.requireSharedMailboxStore(w) {
		return
	}
	mailboxes, err := s.sharedMailboxes.ListMailboxes()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]sharedMailboxView, 0, len(mailboxes))
	for _, m := range mailboxes {
		view, err := s.toSharedMailboxView(m)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		out = append(out, view)
	}
	writeJSON(w, http.StatusOK, out)
}

type sharedMailboxRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// handleCreateSharedMailbox registers an existing mailcow mailbox as a
// shared/team mailbox: the mailbox must already exist and be active, and a
// dedicated mailcow app-password is minted for it immediately (mirroring
// ssoWebmailCredential's mint step exactly), so members never need — or
// learn — the mailbox's real password.
func (s *Server) handleCreateSharedMailbox(w http.ResponseWriter, r *http.Request) {
	if !s.requireSharedMailboxStore(w) {
		return
	}
	var req sharedMailboxRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
		return
	}
	if _, ok, err := s.sharedMailboxes.GetMailboxByEmail(email); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	} else if ok {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "this mailbox is already shared"})
		return
	}

	ctx := r.Context()
	mailboxes, err := s.mc.Mailboxes(ctx)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	if activeMailboxByEmail(mailboxes, email) == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no active mailbox with that address"})
		return
	}

	appPw, appPwID, err := s.mintSharedMailboxAppPassword(ctx, email)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	enc, nonce, err := s.adminCipher.Seal([]byte(appPw))
	if err != nil {
		s.deleteAppPwByName(ctx, email, sharedMailboxAppName(email))
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	id, err := s.sharedMailboxes.CreateMailbox(sharedmailbox.Mailbox{
		Email: email, DisplayName: req.DisplayName, AppPasswdID: appPwID,
		AppPasswdEnc: enc, AppPasswdNonce: nonce, CreatedBy: s.cfg.AdminUser,
	}, time.Now())
	if err != nil {
		s.deleteAppPwByName(ctx, email, sharedMailboxAppName(email))
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	m, _, err := s.sharedMailboxes.GetMailbox(id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	view, err := s.toSharedMailboxView(m)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// sharedMailboxAppName is the fixed, discoverable mailcow app-password name
// for a shared mailbox, so it can be looked up (or compensating-deleted) by
// name alone, exactly like every other app-password minted in this codebase.
func sharedMailboxAppName(email string) string { return "mailfold-shared:" + email }

// mintSharedMailboxAppPassword mints a fresh mailcow app-password scoped to
// IMAP+SMTP for mailbox and returns it along with its mailcow id (needed to
// revoke it later). It is the shared-mailbox analogue of
// ssoWebmailCredential's mint step, kept separate because a shared mailbox
// owns its own dedicated, independently revocable credential rather than
// reusing SSO's cached one for the same mailbox.
func (s *Server) mintSharedMailboxAppPassword(ctx context.Context, email string) (appPw, appPwID string, err error) {
	appPw, err = randomAppPassword()
	if err != nil {
		return "", "", err
	}
	appName := sharedMailboxAppName(email)
	results, err := s.mc.AddAppPassword(ctx, map[string]any{
		"username":    email,
		"app_name":    appName,
		"app_passwd":  appPw,
		"app_passwd2": appPw,
		"active":      "1",
		"protocols":   appPwProtocols,
	})
	if err != nil {
		return "", "", err
	}
	if ok, _ := mailcow.ResultsOK(results); !ok {
		return "", "", errors.New("mailcow rejected app-password creation")
	}
	appPwID, err = s.recoverAppPwID(ctx, email, appName)
	if err != nil {
		s.deleteAppPwByName(ctx, email, appName)
		return "", "", err
	}
	return appPw, appPwID, nil
}

// handleDeleteSharedMailbox un-shares a mailbox: its dedicated app-password
// is revoked at mailcow (best-effort — a failure here is logged, not fatal,
// matching handleAPIKeyRevoke's precedent) and every local row (members,
// assignments, notes) is removed.
func (s *Server) handleDeleteSharedMailbox(w http.ResponseWriter, r *http.Request) {
	if !s.requireSharedMailboxStore(w) {
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	m, ok, err := s.sharedMailboxes.GetMailbox(id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if m.AppPasswdID != "" {
		if _, err := s.mc.DeleteAppPassword(r.Context(), []string{m.AppPasswdID}); err != nil {
			s.logger.Error("shared mailbox deleted locally but mailcow app-password delete failed", "email", m.Email, "error", err)
		}
	}
	if err := s.sharedMailboxes.DeleteMailbox(id); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type sharedMailboxMemberRequest struct {
	MailboxID int64  `json:"mailbox_id"`
	Email     string `json:"email"`
}

// handleAddSharedMailboxMember grants a webmail user access to a shared
// mailbox. The member does not need to already be a Mailfold webmail user —
// access takes effect the next time they sign in and check their shared
// mailboxes.
func (s *Server) handleAddSharedMailboxMember(w http.ResponseWriter, r *http.Request) {
	if !s.requireSharedMailboxStore(w) {
		return
	}
	var req sharedMailboxMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	email := strings.TrimSpace(req.Email)
	if req.MailboxID == 0 || email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mailbox_id and email are required"})
		return
	}
	if _, ok, err := s.sharedMailboxes.GetMailbox(req.MailboxID); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	} else if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err := s.sharedMailboxes.AddMember(req.MailboxID, email, time.Now()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRemoveSharedMailboxMember(w http.ResponseWriter, r *http.Request) {
	if !s.requireSharedMailboxStore(w) {
		return
	}
	var req sharedMailboxMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.MailboxID == 0 || req.Email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mailbox_id and email are required"})
		return
	}
	if err := s.sharedMailboxes.RemoveMember(req.MailboxID, req.Email); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
