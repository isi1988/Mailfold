package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/scheduledsend"
	"github.com/isi1988/Mailfold/backend/internal/webmail"
)

// undoSendWindow is the default delay applied to a "Send" click when the
// caller does not name an explicit send time — long enough to offer a real
// "Undo send" affordance, short enough that the message still leaves quickly.
// It is seeded from config in NewServer's caller (see SetUndoSendWindow) and
// is a package-level var (rather than a Server field) so tests can shorten it
// exactly like webmail_push.go's pushPollInterval / webmail.go's
// webmailEventInterval.
var undoSendWindow = 10 * time.Second

// SetUndoSendWindow overrides the package-level undoSendWindow from
// configuration. It is called once from NewServer.
func SetUndoSendWindow(d time.Duration) {
	if d > 0 {
		undoSendWindow = d
	}
}

// ScheduledSendPollInterval is how often the background dispatcher checks for
// due scheduled sends. It is a var so tests can shorten it, mirroring
// PushPollInterval's exact style.
var scheduledSendPollInterval = 5 * time.Second

// ScheduledSendPollInterval exposes scheduledSendPollInterval to package app,
// which owns the actual background ticker that drives DispatchScheduledSends.
func ScheduledSendPollInterval() time.Duration { return scheduledSendPollInterval }

// scheduledSendStaleAfter bounds how long a row may sit in 'sending' before
// NewServer's startup ResetStale sweep considers it crash-orphaned rather
// than genuinely in flight. A real dispatch (credential resolution + one
// SMTP submission) completes in well under this window, so anything older
// almost certainly means the process died mid-send on a previous run.
const scheduledSendStaleAfter = 5 * time.Minute

// registerWebmailScheduledRoutes wires the send-later/undo-send endpoints.
func (s *Server) registerWebmailScheduledRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/webmail/scheduled", s.requireWebmail(s.handleWebmailScheduleCreate))
	mux.HandleFunc("GET /api/webmail/scheduled", s.requireWebmail(s.handleWebmailScheduleList))
	mux.HandleFunc("DELETE /api/webmail/scheduled/{id}", s.requireWebmail(s.handleWebmailScheduleCancel))
}

// requireScheduledSends reports 501 and returns false unless the feature is
// available (a database and the admin cipher, transitively required by
// ssoWebmailCredential, both configured — see openScheduledSendStore).
func (s *Server) requireScheduledSends(w http.ResponseWriter) bool {
	if s.scheduledSends == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "set MAILFOLD_DB_PATH and MAILFOLD_ADMIN_ENC_KEY to enable send-later/undo-send"})
		return false
	}
	return true
}

// webmailScheduleCreateRequest is the request body for POST
// /api/webmail/scheduled.
type webmailScheduleCreateRequest struct {
	To      []string `json:"to"`
	Cc      []string `json:"cc"`
	Bcc     []string `json:"bcc"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
	HTML    string   `json:"html"`
	SendAt  string   `json:"sendAt"` // optional RFC3339; omitted = implicit undo-send window
}

// handleWebmailScheduleCreate validates and queues a message for future
// delivery: either after the implicit undo-send window (sendAt omitted,
// which is how a normal "Send" click now behaves) or at an explicit future
// instant (sendAt provided, "Send later"). The message is validated eagerly
// — exactly like every other creation endpoint in this codebase — so a
// CRLF-injection attempt is rejected with 400 at creation time rather than
// silently failing when the dispatcher later attempts to send it.
func (s *Server) handleWebmailScheduleCreate(w http.ResponseWriter, r *http.Request) {
	if !s.requireScheduledSends(w) {
		return
	}
	cred := webmailCreds(r)
	var req webmailScheduleCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	msg := webmail.OutgoingMessage{
		To:      req.To,
		Cc:      req.Cc,
		Bcc:     req.Bcc,
		Subject: req.Subject,
		Text:    req.Text,
		HTML:    req.HTML,
	}
	if len(msg.To)+len(msg.Cc)+len(msg.Bcc) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message has no recipients"})
		return
	}
	if err := webmail.ValidateOutgoing(&msg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	now := time.Now()
	scheduledAt := now.Add(undoSendWindow)
	if req.SendAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.SendAt)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sendAt must be an RFC3339 timestamp"})
			return
		}
		if !parsed.After(now) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sendAt must be in the future"})
			return
		}
		scheduledAt = parsed
	}

	id, err := s.scheduledSends.Create(cred.Email, msg, scheduledAt)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":          id,
		"scheduledAt": scheduledAt.UTC().Format(time.RFC3339),
		"status":      scheduledsend.StatusPending,
	})
}

// webmailScheduledItem is one entry in the list response. text/html are
// deliberately omitted — the list view only needs enough to identify and
// cancel an entry, not its full body.
type webmailScheduledItem struct {
	ID          int64    `json:"id"`
	To          []string `json:"to"`
	Cc          []string `json:"cc"`
	Bcc         []string `json:"bcc"`
	Subject     string   `json:"subject"`
	ScheduledAt string   `json:"scheduledAt"`
	Status      string   `json:"status"`
}

// handleWebmailScheduleList returns the caller's own pending/in-flight
// scheduled sends, soonest first.
func (s *Server) handleWebmailScheduleList(w http.ResponseWriter, r *http.Request) {
	if !s.requireScheduledSends(w) {
		return
	}
	cred := webmailCreds(r)
	rows, err := s.scheduledSends.ListPending(cred.Email)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]webmailScheduledItem, len(rows))
	for i, row := range rows {
		out[i] = webmailScheduledItem{
			ID:          row.ID,
			To:          row.To,
			Cc:          row.Cc,
			Bcc:         row.Bcc,
			Subject:     row.Subject,
			ScheduledAt: row.ScheduledAt.UTC().Format(time.RFC3339),
			Status:      row.Status,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleWebmailScheduleCancel cancels a still-pending scheduled send owned by
// the caller.
//
// Distinguishing "not found" from "already sending/terminal" would require an
// extra lookup query beyond the single scoped UPDATE Cancel already runs; per
// the contract's nice-to-have note, this implementation takes the simpler
// 404-for-both approach rather than paying for that extra query on every
// cancel attempt.
func (s *Server) handleWebmailScheduleCancel(w http.ResponseWriter, r *http.Request) {
	if !s.requireScheduledSends(w) {
		return
	}
	cred := webmailCreds(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	ok, err := s.scheduledSends.Cancel(cred.Email, id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": scheduledsend.StatusCanceled})
}

// DispatchScheduledSends claims every due scheduled send (up to one batch)
// and attempts delivery for each, never letting one row's failure abort the
// rest of the batch. It is a no-op when the feature is disabled, and is
// intended to be called periodically from a background goroutine — see
// ScheduledSendPollInterval.
func (s *Server) DispatchScheduledSends() {
	if s.scheduledSends == nil {
		return
	}
	claimed, err := s.scheduledSends.ClaimDue(time.Now(), 20)
	if err != nil {
		s.logger.Error("scheduled-send: claim due rows failed", "error", err)
		return
	}
	ctx := context.Background()
	for _, row := range claimed {
		s.dispatchOneScheduledSend(ctx, row)
	}
}

// dispatchOneScheduledSend resolves a working credential for row's owner and
// attempts to send it, recording the outcome. A failure here never touches
// any other claimed row.
func (s *Server) dispatchOneScheduledSend(ctx context.Context, row scheduledsend.ScheduledSend) {
	appPw, err := s.ssoWebmailCredential(ctx, row.OwnerEmail)
	if err != nil {
		s.logger.Error("scheduled-send: failed to obtain a webmail credential", "id", row.ID, "email", row.OwnerEmail, "error", err)
		if mErr := s.scheduledSends.MarkFailed(row.ID, err.Error()); mErr != nil {
			s.logger.Error("scheduled-send: mark failed failed", "id", row.ID, "error", mErr)
		}
		return
	}

	msg := webmail.OutgoingMessage{
		To:      row.To,
		Cc:      row.Cc,
		Bcc:     row.Bcc,
		Subject: row.Subject,
		Text:    row.Text,
		HTML:    row.HTML,
	}
	if err := s.webmail.Send(row.OwnerEmail, appPw, &msg); err != nil {
		s.logger.Error("scheduled-send: send failed", "id", row.ID, "email", row.OwnerEmail, "error", err)
		if mErr := s.scheduledSends.MarkFailed(row.ID, err.Error()); mErr != nil {
			s.logger.Error("scheduled-send: mark failed failed", "id", row.ID, "error", mErr)
		}
		return
	}
	// Save a copy to the Sent folder. Best-effort: the mail has already been
	// submitted, so a failed copy is logged, not treated as a send failure —
	// mirrors handleWebmailSend's own SaveToSent call exactly.
	if err := s.webmail.SaveToSent(row.OwnerEmail, appPw, &msg); err != nil {
		s.logger.Warn("scheduled-send: message sent but not saved to Sent folder", "id", row.ID, "email", row.OwnerEmail, "error", err)
	}
	if err := s.scheduledSends.MarkSent(row.ID); err != nil {
		s.logger.Error("scheduled-send: mark sent failed", "id", row.ID, "error", err)
	}
}
