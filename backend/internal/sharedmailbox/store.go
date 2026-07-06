// Package sharedmailbox persists team/shared mailboxes: a mailcow mailbox
// (e.g. support@company.com) that more than one webmail user can sign into,
// with a per-message assignment (who on the team is handling it) and a
// thread of internal notes (visible to the team only, never sent to the
// actual sender or recipient).
//
// A shared mailbox's own real password is never learned by its members: the
// api package mints a mailcow app-password for it once (see
// internal/api/webmail_shared.go, mirroring internal/api/auth_sso.go's
// ssoWebmailCredential exactly) and this package stores the encrypted
// result, using the same admin cipher SSO's cached mailbox credential
// already uses — both are the identical kind of secret (a mailcow
// app-password standing in for a mailbox's real password), so there is no
// reason for this package to own a second cipher.
//
// It runs on the same database as the admin/DAV/API-key/webmail-user/
// domain-admin stores, following their exact Open/migrate/Dialect pattern.
package sharedmailbox

import (
	"database/sql"
	"time"

	"github.com/isi1988/Mailfold/backend/storage"
)

// Mailbox is one shared mailbox: the underlying mailcow mailbox it fronts,
// plus the mailcow app-password (encrypted) every member's delegated
// session authenticates with.
type Mailbox struct {
	ID             int64
	Email          string
	DisplayName    string
	AppPasswdID    string
	AppPasswdEnc   []byte
	AppPasswdNonce []byte
	CreatedBy      string
	CreatedAt      time.Time
}

// Note is one internal note left on a message within a shared mailbox,
// identified by folder + IMAP UID (stable within a folder, like every other
// per-message reference in this codebase).
type Note struct {
	ID          int64
	MailboxID   int64
	Folder      string
	UID         uint32
	AuthorEmail string
	Body        string
	CreatedAt   time.Time
}

// Store is the persistence layer for shared mailboxes, their members, and
// per-message assignment/notes.
type Store struct {
	db *sql.DB
	d  storage.Dialect
}

// Open opens the database on the given driver and DSN and applies the schema.
func Open(driver, dsn string) (*Store, error) {
	db, err := storage.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db.DB, d: db.Dialect}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS shared_mailbox (
    id                 ` + s.d.IntType() + ` PRIMARY KEY,
    email              TEXT NOT NULL UNIQUE,
    display_name       TEXT NOT NULL DEFAULT '',
    app_passwd_id      TEXT NOT NULL DEFAULT '',
    app_passwd_enc     ` + s.d.BlobType() + `,
    app_passwd_nonce   ` + s.d.BlobType() + `,
    created_by         TEXT NOT NULL DEFAULT '',
    created_at         ` + s.d.IntType() + ` NOT NULL DEFAULT 0
)`,
		`CREATE TABLE IF NOT EXISTS shared_mailbox_member (
    mailbox_id  ` + s.d.IntType() + ` NOT NULL,
    email       TEXT NOT NULL,
    added_at    ` + s.d.IntType() + ` NOT NULL DEFAULT 0,
    PRIMARY KEY (mailbox_id, email)
)`,
		`CREATE TABLE IF NOT EXISTS shared_mailbox_assignment (
    mailbox_id    ` + s.d.IntType() + ` NOT NULL,
    folder        TEXT NOT NULL,
    uid           ` + s.d.IntType() + ` NOT NULL,
    assigned_to   TEXT NOT NULL,
    assigned_by   TEXT NOT NULL,
    assigned_at   ` + s.d.IntType() + ` NOT NULL DEFAULT 0,
    PRIMARY KEY (mailbox_id, folder, uid)
)`,
		`CREATE TABLE IF NOT EXISTS shared_mailbox_note (
    id             ` + s.d.IntType() + ` PRIMARY KEY,
    mailbox_id     ` + s.d.IntType() + ` NOT NULL,
    folder         TEXT NOT NULL,
    uid            ` + s.d.IntType() + ` NOT NULL,
    author_email   TEXT NOT NULL,
    body           TEXT NOT NULL,
    created_at     ` + s.d.IntType() + ` NOT NULL DEFAULT 0
)`,
		`CREATE INDEX IF NOT EXISTS idx_shared_mailbox_note_msg ON shared_mailbox_note (mailbox_id, folder, uid)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) exec(query string, args ...any) (sql.Result, error) {
	return s.db.Exec(s.d.Rebind(query), args...)
}

// --- Mailboxes ---

// CreateMailbox inserts a new shared mailbox and returns its id.
func (s *Store) CreateMailbox(m Mailbox, now time.Time) (int64, error) {
	res, err := s.exec(`INSERT INTO shared_mailbox (email, display_name, app_passwd_id, app_passwd_enc, app_passwd_nonce, created_by, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.Email, m.DisplayName, m.AppPasswdID, m.AppPasswdEnc, m.AppPasswdNonce, m.CreatedBy, storage.Unix(now))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

const mailboxColumns = `id, email, display_name, app_passwd_id, app_passwd_enc, app_passwd_nonce, created_by, created_at`

func scanMailboxRow(row scanner) (Mailbox, error) {
	var m Mailbox
	var createdAt int64
	err := row.Scan(&m.ID, &m.Email, &m.DisplayName, &m.AppPasswdID, &m.AppPasswdEnc, &m.AppPasswdNonce, &m.CreatedBy, &createdAt)
	if err != nil {
		return Mailbox{}, err
	}
	m.CreatedAt = storage.FromUnix(createdAt)
	return m, nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

// GetMailbox returns a shared mailbox by id.
func (s *Store) GetMailbox(id int64) (Mailbox, bool, error) {
	row := s.db.QueryRow(s.d.Rebind(`SELECT `+mailboxColumns+` FROM shared_mailbox WHERE id = ?`), id)
	m, err := scanMailboxRow(row)
	if err == sql.ErrNoRows {
		return Mailbox{}, false, nil
	}
	if err != nil {
		return Mailbox{}, false, err
	}
	return m, true, nil
}

// GetMailboxByEmail returns a shared mailbox by its underlying mailbox
// address.
func (s *Store) GetMailboxByEmail(email string) (Mailbox, bool, error) {
	row := s.db.QueryRow(s.d.Rebind(`SELECT `+mailboxColumns+` FROM shared_mailbox WHERE email = ?`), email)
	m, err := scanMailboxRow(row)
	if err == sql.ErrNoRows {
		return Mailbox{}, false, nil
	}
	if err != nil {
		return Mailbox{}, false, err
	}
	return m, true, nil
}

// ListMailboxes returns every shared mailbox, most recently created first.
func (s *Store) ListMailboxes() ([]Mailbox, error) {
	rows, err := s.db.Query(`SELECT ` + mailboxColumns + ` FROM shared_mailbox ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Mailbox
	for rows.Next() {
		m, err := scanMailboxRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MailboxesForMember returns every shared mailbox member has been granted
// access to.
func (s *Store) MailboxesForMember(member string) ([]Mailbox, error) {
	rows, err := s.db.Query(s.d.Rebind(`
        SELECT `+mailboxColumns+` FROM shared_mailbox
        WHERE id IN (SELECT mailbox_id FROM shared_mailbox_member WHERE email = ?)
        ORDER BY created_at DESC`), member)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Mailbox
	for rows.Next() {
		m, err := scanMailboxRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DeleteMailbox removes a shared mailbox and every member/assignment/note
// scoped to it. The caller is responsible for revoking its mailcow
// app-password beforehand — this only touches Mailfold's own tables.
func (s *Store) DeleteMailbox(id int64) error {
	for _, stmt := range []string{
		`DELETE FROM shared_mailbox_member WHERE mailbox_id = ?`,
		`DELETE FROM shared_mailbox_assignment WHERE mailbox_id = ?`,
		`DELETE FROM shared_mailbox_note WHERE mailbox_id = ?`,
		`DELETE FROM shared_mailbox WHERE id = ?`,
	} {
		if _, err := s.exec(stmt, id); err != nil {
			return err
		}
	}
	return nil
}

// --- Members ---

// AddMember grants member access to a shared mailbox. Idempotent: adding an
// existing member just refreshes added_at.
func (s *Store) AddMember(mailboxID int64, member string, now time.Time) error {
	_, err := s.exec(`INSERT INTO shared_mailbox_member (mailbox_id, email, added_at) VALUES (?, ?, ?)
        ON CONFLICT(mailbox_id, email) DO UPDATE SET added_at = excluded.added_at`,
		mailboxID, member, storage.Unix(now))
	return err
}

// RemoveMember revokes member's access to a shared mailbox.
func (s *Store) RemoveMember(mailboxID int64, member string) error {
	_, err := s.exec(`DELETE FROM shared_mailbox_member WHERE mailbox_id = ? AND email = ?`, mailboxID, member)
	return err
}

// ListMembers returns every member's email, alphabetically.
func (s *Store) ListMembers(mailboxID int64) ([]string, error) {
	rows, err := s.db.Query(s.d.Rebind(`SELECT email FROM shared_mailbox_member WHERE mailbox_id = ? ORDER BY email`), mailboxID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		out = append(out, email)
	}
	return out, rows.Err()
}

// IsMember reports whether member has been granted access to mailboxID.
func (s *Store) IsMember(mailboxID int64, member string) (bool, error) {
	var one int
	err := s.db.QueryRow(s.d.Rebind(`SELECT 1 FROM shared_mailbox_member WHERE mailbox_id = ? AND email = ?`), mailboxID, member).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// --- Assignments ---

// SetAssignment assigns a message to assignedTo (upserting any existing
// assignment), recording who made the change.
func (s *Store) SetAssignment(mailboxID int64, folder string, uid uint32, assignedTo, assignedBy string, now time.Time) error {
	_, err := s.exec(`INSERT INTO shared_mailbox_assignment (mailbox_id, folder, uid, assigned_to, assigned_by, assigned_at)
        VALUES (?, ?, ?, ?, ?, ?)
        ON CONFLICT(mailbox_id, folder, uid) DO UPDATE SET assigned_to = excluded.assigned_to,
            assigned_by = excluded.assigned_by, assigned_at = excluded.assigned_at`,
		mailboxID, folder, uid, assignedTo, assignedBy, storage.Unix(now))
	return err
}

// ClearAssignment un-assigns a message.
func (s *Store) ClearAssignment(mailboxID int64, folder string, uid uint32) error {
	_, err := s.exec(`DELETE FROM shared_mailbox_assignment WHERE mailbox_id = ? AND folder = ? AND uid = ?`, mailboxID, folder, uid)
	return err
}

// GetAssignment returns who a message is assigned to, if anyone.
func (s *Store) GetAssignment(mailboxID int64, folder string, uid uint32) (string, bool, error) {
	var assignedTo string
	err := s.db.QueryRow(s.d.Rebind(`SELECT assigned_to FROM shared_mailbox_assignment WHERE mailbox_id = ? AND folder = ? AND uid = ?`),
		mailboxID, folder, uid).Scan(&assignedTo)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return assignedTo, true, nil
}

// AssignmentsForFolder returns every assignment in folder as uid -> assignee,
// for enriching a message list in one query rather than one per message.
func (s *Store) AssignmentsForFolder(mailboxID int64, folder string) (map[uint32]string, error) {
	rows, err := s.db.Query(s.d.Rebind(`SELECT uid, assigned_to FROM shared_mailbox_assignment WHERE mailbox_id = ? AND folder = ?`), mailboxID, folder)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[uint32]string{}
	for rows.Next() {
		var uid uint32
		var assignedTo string
		if err := rows.Scan(&uid, &assignedTo); err != nil {
			return nil, err
		}
		out[uid] = assignedTo
	}
	return out, rows.Err()
}

// --- Notes ---

// AddNote appends a note to a message and returns the stored row.
func (s *Store) AddNote(mailboxID int64, folder string, uid uint32, author, body string, now time.Time) (Note, error) {
	res, err := s.exec(`INSERT INTO shared_mailbox_note (mailbox_id, folder, uid, author_email, body, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		mailboxID, folder, uid, author, body, storage.Unix(now))
	if err != nil {
		return Note{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Note{}, err
	}
	return Note{ID: id, MailboxID: mailboxID, Folder: folder, UID: uid, AuthorEmail: author, Body: body, CreatedAt: now}, nil
}

// ListNotes returns every note on a message, oldest first.
func (s *Store) ListNotes(mailboxID int64, folder string, uid uint32) ([]Note, error) {
	rows, err := s.db.Query(s.d.Rebind(`SELECT id, author_email, body, created_at FROM shared_mailbox_note
        WHERE mailbox_id = ? AND folder = ? AND uid = ? ORDER BY created_at, id`), mailboxID, folder, uid)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Note
	for rows.Next() {
		n := Note{MailboxID: mailboxID, Folder: folder, UID: uid}
		var createdAt int64
		if err := rows.Scan(&n.ID, &n.AuthorEmail, &n.Body, &createdAt); err != nil {
			return nil, err
		}
		n.CreatedAt = storage.FromUnix(createdAt)
		out = append(out, n)
	}
	return out, rows.Err()
}

// GetNote returns a single note by id, for an ownership check before delete.
func (s *Store) GetNote(id int64) (Note, bool, error) {
	var n Note
	var createdAt int64
	err := s.db.QueryRow(s.d.Rebind(`SELECT id, mailbox_id, folder, uid, author_email, body, created_at FROM shared_mailbox_note WHERE id = ?`), id).
		Scan(&n.ID, &n.MailboxID, &n.Folder, &n.UID, &n.AuthorEmail, &n.Body, &createdAt)
	if err == sql.ErrNoRows {
		return Note{}, false, nil
	}
	if err != nil {
		return Note{}, false, err
	}
	n.CreatedAt = storage.FromUnix(createdAt)
	return n, true, nil
}

// DeleteNote removes a note by id.
func (s *Store) DeleteNote(id int64) error {
	_, err := s.exec(`DELETE FROM shared_mailbox_note WHERE id = ?`, id)
	return err
}

// NoteCountsForFolder returns how many notes exist per message in folder, as
// uid -> count, for enriching a message list in one query.
func (s *Store) NoteCountsForFolder(mailboxID int64, folder string) (map[uint32]int, error) {
	rows, err := s.db.Query(s.d.Rebind(`SELECT uid, COUNT(*) FROM shared_mailbox_note WHERE mailbox_id = ? AND folder = ? GROUP BY uid`), mailboxID, folder)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[uint32]int{}
	for rows.Next() {
		var uid uint32
		var count int
		if err := rows.Scan(&uid, &count); err != nil {
			return nil, err
		}
		out[uid] = count
	}
	return out, rows.Err()
}
