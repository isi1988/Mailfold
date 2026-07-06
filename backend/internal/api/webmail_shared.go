package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/sharedmailbox"
	"github.com/isi1988/Mailfold/backend/internal/webmail"
)

// registerWebmailSharedRoutes wires the member-facing side of shared/team
// mailboxes: discovering which shared mailboxes the signed-in mailbox has
// been granted access to, activating one as an ordinary webmail session (see
// activateSharedMailboxSession), and per-message assignment/notes once
// inside one. The super-admin's side — creating a shared mailbox and
// granting/revoking member access — is registered separately in
// shared_mailboxes_admin.go.
func (s *Server) registerWebmailSharedRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/webmail/shared", s.requireWebmail(s.handleWebmailSharedMailboxes))
	mux.HandleFunc("POST /api/webmail/shared/{id}/session", s.requireWebmail(s.handleWebmailSharedSession))
	mux.HandleFunc("GET /api/webmail/shared/members", s.requireWebmail(s.handleWebmailSharedMembers))
	mux.HandleFunc("GET /api/webmail/shared/assignment", s.requireWebmail(s.handleWebmailGetAssignment))
	mux.HandleFunc("POST /api/webmail/shared/assignment", s.requireWebmail(s.handleWebmailSetAssignment))
	mux.HandleFunc("GET /api/webmail/shared/notes", s.requireWebmail(s.handleWebmailListNotes))
	mux.HandleFunc("POST /api/webmail/shared/notes", s.requireWebmail(s.handleWebmailAddNote))
	mux.HandleFunc("DELETE /api/webmail/shared/notes", s.requireWebmail(s.handleWebmailDeleteNote))
}

// sharedMailboxOption is the wire shape of a shared mailbox as offered to a
// member — no app-password, no other members, just enough to activate it.
type sharedMailboxOption struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// handleWebmailSharedMailboxes lists the shared mailboxes the signed-in
// mailbox has been granted access to, empty (not an error) when the feature
// is disabled — there is nothing to offer, that's all, matching the SSO
// provider-discovery endpoint's precedent.
func (s *Server) handleWebmailSharedMailboxes(w http.ResponseWriter, r *http.Request) {
	if s.sharedMailboxes == nil {
		writeJSON(w, http.StatusOK, []sharedMailboxOption{})
		return
	}
	cred := webmailCreds(r)
	mailboxes, err := s.sharedMailboxes.MailboxesForMember(cred.Email)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]sharedMailboxOption, 0, len(mailboxes))
	for _, m := range mailboxes {
		out = append(out, sharedMailboxOption{ID: m.ID, Email: m.Email, DisplayName: m.DisplayName})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleWebmailSharedSession activates a shared mailbox the caller has been
// granted access to: it mints a fresh webmail session for the shared
// mailbox itself, authenticated with its delegated app-password (the caller
// never sees it) and tagged with ActingAs so subsequent assignment/notes
// calls from that session attribute back to the real person. This mirrors
// handleSSOCallback minting a webmail session straight from a verified
// identity, without the mailbox's own TOTP (if any) standing in the way —
// access was already gated by shared-mailbox membership, checked below.
func (s *Server) handleWebmailSharedSession(w http.ResponseWriter, r *http.Request) {
	if !s.requireSharedMailboxStore(w) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	cred := webmailCreds(r)
	m, ok, err := s.sharedMailboxes.GetMailbox(id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if isMember, err := s.sharedMailboxes.IsMember(id, cred.Email); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	} else if !isMember {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "you do not have access to this shared mailbox"})
		return
	}
	appPw, err := s.adminCipher.Open(m.AppPasswdEnc, m.AppPasswdNonce)
	if err != nil {
		s.logger.Error("shared mailbox credential could not be decrypted", "email", m.Email, "error", err)
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	token, exp, err := s.webmailSessions.CreateActingAs(m.Email, string(appPw), cred.Email)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "email": m.Email, "expires_at": exp})
}

// resolveSharedSession looks up the shared mailbox the CURRENT session
// belongs to (by its own mailbox email, regardless of how it was minted) and
// the identity to attribute assignments/notes to: ActingAs when the session
// was activated through handleWebmailSharedSession above, or the session's
// own email when someone signed into the shared mailbox directly with its
// real password (an unusual but valid path — the underlying mailbox's owner
// may still know it). ok is false when this session's mailbox is not a
// shared mailbox at all, in which case every handler below reports 404
// rather than leaking whether the address happens to be a plain mailbox.
func (s *Server) resolveSharedSession(r *http.Request) (mailbox sharedmailbox.Mailbox, actingAs string, ok bool, err error) {
	if s.sharedMailboxes == nil {
		return sharedmailbox.Mailbox{}, "", false, nil
	}
	cred := webmailCreds(r)
	m, found, err := s.sharedMailboxes.GetMailboxByEmail(cred.Email)
	if err != nil || !found {
		return sharedmailbox.Mailbox{}, "", false, err
	}
	actingAs = cred.ActingAs
	if actingAs == "" {
		actingAs = cred.Email
	}
	return m, actingAs, true, nil
}

// errNotSharedMailbox is the uniform 404 body every per-message handler
// below reports when resolveSharedSession finds this session's mailbox is
// not a shared mailbox — see its doc comment for why that's 404, not 403.
const errNotSharedMailbox = "this mailbox is not a shared mailbox"

// requireSharedSession is resolveSharedSession plus writing the appropriate
// error response and reporting ok=false when the caller should stop: a 500
// on a lookup failure, or the 404 above when this isn't a shared mailbox.
func (s *Server) requireSharedSession(w http.ResponseWriter, r *http.Request) (sharedmailbox.Mailbox, string, bool) {
	m, actingAs, ok, err := s.resolveSharedSession(r)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return sharedmailbox.Mailbox{}, "", false
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": errNotSharedMailbox})
		return sharedmailbox.Mailbox{}, "", false
	}
	return m, actingAs, true
}

func (s *Server) handleWebmailSharedMembers(w http.ResponseWriter, r *http.Request) {
	m, _, ok := s.requireSharedSession(w, r)
	if !ok {
		return
	}
	members, err := s.sharedMailboxes.ListMembers(m.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, members)
}

type assignmentView struct {
	AssignedTo string `json:"assigned_to"`
}

func (s *Server) handleWebmailGetAssignment(w http.ResponseWriter, r *http.Request) {
	m, _, ok := s.requireSharedSession(w, r)
	if !ok {
		return
	}
	uid, err := strconv.ParseUint(r.URL.Query().Get("uid"), 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid uid"})
		return
	}
	assignedTo, _, err := s.sharedMailboxes.GetAssignment(m.ID, folderParam(r), uint32(uid))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, assignmentView{AssignedTo: assignedTo})
}

type setAssignmentRequest struct {
	Folder     string `json:"folder"`
	UID        uint32 `json:"uid"`
	AssignedTo string `json:"assigned_to"`
}

// handleWebmailSetAssignment assigns (or, with an empty assigned_to, clears)
// a message, restricted to the shared mailbox's own members — assigning to
// someone with no access to the mailbox would silently create a task no one
// can act on.
func (s *Server) handleWebmailSetAssignment(w http.ResponseWriter, r *http.Request) {
	m, actingAs, ok := s.requireSharedSession(w, r)
	if !ok {
		return
	}
	var req setAssignmentRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	folder := orInbox(req.Folder)
	assignedTo := strings.TrimSpace(req.AssignedTo)
	if assignedTo == "" {
		if err := s.sharedMailboxes.ClearAssignment(m.ID, folder, req.UID); err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if isMember, err := s.sharedMailboxes.IsMember(m.ID, assignedTo); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	} else if !isMember {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "assigned_to is not a member of this shared mailbox"})
		return
	}
	if err := s.sharedMailboxes.SetAssignment(m.ID, folder, req.UID, assignedTo, actingAs, time.Now()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type noteView struct {
	ID        int64     `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

func toNoteView(n sharedmailbox.Note) noteView {
	return noteView{ID: n.ID, Author: n.AuthorEmail, Body: n.Body, CreatedAt: n.CreatedAt}
}

func (s *Server) handleWebmailListNotes(w http.ResponseWriter, r *http.Request) {
	m, _, ok := s.requireSharedSession(w, r)
	if !ok {
		return
	}
	uid, err := strconv.ParseUint(r.URL.Query().Get("uid"), 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid uid"})
		return
	}
	notes, err := s.sharedMailboxes.ListNotes(m.ID, folderParam(r), uint32(uid))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]noteView, len(notes))
	for i, n := range notes {
		out[i] = toNoteView(n)
	}
	writeJSON(w, http.StatusOK, out)
}

type addNoteRequest struct {
	Folder string `json:"folder"`
	UID    uint32 `json:"uid"`
	Body   string `json:"body"`
}

func (s *Server) handleWebmailAddNote(w http.ResponseWriter, r *http.Request) {
	m, actingAs, ok := s.requireSharedSession(w, r)
	if !ok {
		return
	}
	var req addNoteRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body is required"})
		return
	}
	n, err := s.sharedMailboxes.AddNote(m.ID, orInbox(req.Folder), req.UID, actingAs, body, time.Now())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, toNoteView(n))
}

// handleWebmailDeleteNote removes a note. Only its own author may delete it
// — an internal note is that person's own record, not something a teammate
// should be able to erase.
func (s *Server) handleWebmailDeleteNote(w http.ResponseWriter, r *http.Request) {
	m, actingAs, ok := s.requireSharedSession(w, r)
	if !ok {
		return
	}
	var req struct {
		ID int64 `json:"id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	n, found, err := s.sharedMailboxes.GetNote(req.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !found || n.MailboxID != m.ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if n.AuthorEmail != actingAs {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "only the author can delete this note"})
		return
	}
	if err := s.sharedMailboxes.DeleteNote(req.ID); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// enrichShared fills AssignedTo/NotesCount on every header in msgs when the
// current session belongs to a shared mailbox, in two queries total
// regardless of how many messages are listed — never per-message. It is a
// no-op (and cheap: one GetMailboxByEmail lookup) for an ordinary mailbox
// session.
func (s *Server) enrichShared(r *http.Request, folder string, msgs []webmail.MessageHeader) {
	m, _, ok, err := s.resolveSharedSession(r)
	if err != nil || !ok {
		return
	}
	assignments, err := s.sharedMailboxes.AssignmentsForFolder(m.ID, folder)
	if err != nil {
		return
	}
	counts, err := s.sharedMailboxes.NoteCountsForFolder(m.ID, folder)
	if err != nil {
		return
	}
	for i := range msgs {
		msgs[i].AssignedTo = assignments[msgs[i].UID]
		msgs[i].NotesCount = counts[msgs[i].UID]
	}
}
