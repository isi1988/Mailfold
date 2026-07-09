// Package scheduledsend persists "send later" / "undo send" outgoing
// messages: a message a webmail user has composed but wants delivered at a
// future instant (either an explicit time they picked, or a short implicit
// grace period that gives them a window to cancel a message that otherwise
// looks "sent" from the compose UI's point of view).
//
// It never stores the mailbox's real plaintext password: at dispatch time the
// api package resolves a working credential via its existing
// ssoWebmailCredential helper (the same cached mailcow app-password SSO
// logins already use), so this package only ever persists the message
// content and scheduling metadata.
//
// It runs on the same database as the admin/DAV/API-key/webmail-user/
// domain-admin/shared-mailbox stores, following their exact
// Open/migrate/Dialect pattern (see internal/sharedmailbox/store.go).
package scheduledsend

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/webmail"
	"github.com/isi1988/Mailfold/backend/storage"
)

// Status values for a scheduled_send row.
const (
	StatusPending  = "pending"
	StatusSending  = "sending"
	StatusSent     = "sent"
	StatusCanceled = "canceled"
	StatusFailed   = "failed"
)

// ScheduledSend is one queued outgoing message.
type ScheduledSend struct {
	ID          int64
	OwnerEmail  string
	To          []string
	Cc          []string
	Bcc         []string
	Subject     string
	Text        string
	HTML        string
	ScheduledAt time.Time
	Status      string
	CreatedAt   time.Time
	ClaimedAt   time.Time // zero unless Status is (or was) 'sending'; see ResetStale
	LastError   string
}

// Store is the persistence layer for scheduled/undo-send messages.
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
		`CREATE TABLE IF NOT EXISTS scheduled_send (
    id            ` + s.d.IntType() + ` PRIMARY KEY,
    owner_email   TEXT NOT NULL,
    to_json       TEXT NOT NULL,
    cc_json       TEXT NOT NULL DEFAULT '[]',
    bcc_json      TEXT NOT NULL DEFAULT '[]',
    subject       TEXT NOT NULL DEFAULT '',
    text_body     TEXT NOT NULL DEFAULT '',
    html_body     TEXT NOT NULL DEFAULT '',
    scheduled_at  ` + s.d.IntType() + ` NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending',
    created_at    ` + s.d.IntType() + ` NOT NULL,
    claimed_at    ` + s.d.IntType() + ` NOT NULL DEFAULT 0,
    last_error    TEXT NOT NULL DEFAULT ''
)`,
		`CREATE INDEX IF NOT EXISTS idx_scheduled_send_status_time ON scheduled_send (status, scheduled_at)`,
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

const columns = `id, owner_email, to_json, cc_json, bcc_json, subject, text_body, html_body, scheduled_at, status, created_at, claimed_at, last_error`

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanRow(row scanner) (ScheduledSend, error) {
	var r ScheduledSend
	var toJSON, ccJSON, bccJSON string
	var scheduledAt, createdAt, claimedAt int64
	err := row.Scan(&r.ID, &r.OwnerEmail, &toJSON, &ccJSON, &bccJSON, &r.Subject, &r.Text, &r.HTML,
		&scheduledAt, &r.Status, &createdAt, &claimedAt, &r.LastError)
	if err != nil {
		return ScheduledSend{}, err
	}
	r.To = decodeAddrList(toJSON)
	r.Cc = decodeAddrList(ccJSON)
	r.Bcc = decodeAddrList(bccJSON)
	r.ScheduledAt = storage.FromUnix(scheduledAt)
	r.CreatedAt = storage.FromUnix(createdAt)
	if claimedAt > 0 {
		r.ClaimedAt = storage.FromUnix(claimedAt)
	}
	return r, nil
}

func decodeAddrList(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func encodeAddrList(list []string) (string, error) {
	if list == nil {
		list = []string{}
	}
	b, err := json.Marshal(list)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Create inserts a new scheduled send owned by owner and returns its id.
func (s *Store) Create(owner string, msg webmail.OutgoingMessage, scheduledAt time.Time) (int64, error) {
	toJSON, err := encodeAddrList(msg.To)
	if err != nil {
		return 0, err
	}
	ccJSON, err := encodeAddrList(msg.Cc)
	if err != nil {
		return 0, err
	}
	bccJSON, err := encodeAddrList(msg.Bcc)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	res, err := s.exec(`INSERT INTO scheduled_send
        (owner_email, to_json, cc_json, bcc_json, subject, text_body, html_body, scheduled_at, status, created_at, claimed_at, last_error)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		owner, toJSON, ccJSON, bccJSON, msg.Subject, msg.Text, msg.HTML,
		storage.Unix(scheduledAt), StatusPending, storage.Unix(now), 0, "")
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListPending returns every pending or in-flight ("sending") scheduled send
// belonging to owner, soonest first.
func (s *Store) ListPending(owner string) ([]ScheduledSend, error) {
	rows, err := s.db.Query(s.d.Rebind(`SELECT `+columns+` FROM scheduled_send
        WHERE owner_email = ? AND status IN ('pending', 'sending')
        ORDER BY scheduled_at ASC`), owner)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []ScheduledSend
	for rows.Next() {
		r, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Cancel transitions a row from 'pending' to 'canceled', scoped to the given
// owner and id. It reports ok=false (not an error) when the row does not
// exist, is not owned by owner, or is no longer 'pending' (already claimed by
// the dispatcher or already terminal) — the caller turns that into a
// 404/409.
func (s *Store) Cancel(owner string, id int64) (bool, error) {
	res, err := s.exec(`UPDATE scheduled_send SET status = ? WHERE id = ? AND owner_email = ? AND status = ?`,
		StatusCanceled, id, owner, StatusPending)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ClaimDue atomically claims up to limit rows that are due (status='pending'
// and scheduled_at <= now), soonest first, transitioning them to 'sending'
// and returning the claimed rows in full.
//
// This runs inside one transaction: select the candidate ids, then update
// exactly those ids that are still 'pending' (guarding against any change
// between the select and the update), then commit and return what was
// actually claimed. The codebase's dispatcher is a single sequential ticker
// loop with no concurrent callers, so this is intentionally not built for
// distributed/multi-process locking — but the SELECT-then-UPDATE-still-
// pending pattern inside one transaction does guarantee a row claimed once
// cannot be claimed again by a later tick even if an earlier tick's send is
// still in flight (it is no longer 'pending' by then), which is the actual
// safety property required.
func (s *Store) ClaimDue(now time.Time, limit int) ([]ScheduledSend, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	ids, err := selectDueIDs(tx, s.d, now, limit)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		committed = true
		return nil, nil
	}

	if err := claimIDs(tx, s.d, ids, now); err != nil {
		return nil, err
	}
	claimed, err := loadRowsByID(tx, s.d, ids)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return claimed, nil
}

// selectDueIDs returns the ids of every 'pending' row whose scheduled_at has
// arrived, soonest first, capped at limit.
func selectDueIDs(tx *sql.Tx, d storage.Dialect, now time.Time, limit int) ([]int64, error) {
	rows, err := tx.Query(d.Rebind(`SELECT id FROM scheduled_send
        WHERE status = ? AND scheduled_at <= ?
        ORDER BY scheduled_at ASC LIMIT ?`), StatusPending, storage.Unix(now), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// claimIDs transitions exactly the given ids from 'pending' to 'sending',
// stamping claimed_at. The WHERE ... AND status = 'pending' guard is what
// makes a claim safe to repeat: a row already moved off 'pending' by another
// caller simply matches zero rows here instead of being claimed twice.
func claimIDs(tx *sql.Tx, d storage.Dialect, ids []int64, now time.Time) error {
	for _, id := range ids {
		if _, err := tx.Exec(d.Rebind(`UPDATE scheduled_send SET status = ?, claimed_at = ? WHERE id = ? AND status = ?`),
			StatusSending, storage.Unix(now), id, StatusPending); err != nil {
			return err
		}
	}
	return nil
}

// loadRowsByID re-reads each id in full, in the order given.
func loadRowsByID(tx *sql.Tx, d storage.Dialect, ids []int64) ([]ScheduledSend, error) {
	out := make([]ScheduledSend, 0, len(ids))
	for _, id := range ids {
		row := tx.QueryRow(d.Rebind(`SELECT `+columns+` FROM scheduled_send WHERE id = ?`), id)
		r, err := scanRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// MarkSent transitions a claimed row to 'sent'.
func (s *Store) MarkSent(id int64) error {
	_, err := s.exec(`UPDATE scheduled_send SET status = ? WHERE id = ?`, StatusSent, id)
	return err
}

// MarkFailed transitions a claimed row to 'failed', recording errMsg.
func (s *Store) MarkFailed(id int64, errMsg string) error {
	_, err := s.exec(`UPDATE scheduled_send SET status = ?, last_error = ? WHERE id = ?`, StatusFailed, errMsg, id)
	return err
}

// ResetStale resets rows stuck in 'sending' with claimed_at older than
// olderThan back to 'pending', self-healing rows orphaned by a crash mid-send
// (the process died between ClaimDue's UPDATE and the subsequent MarkSent/
// MarkFailed).
//
// This MUST compare against claimed_at (when ClaimDue flipped the row to
// 'sending'), never created_at (when the row was first scheduled): a "send
// later" message can sit 'pending' for hours or days before it is ever
// claimed, so created_at is old by the time it is legitimately claimed and
// sent. An earlier version of this function compared against created_at and
// could reset a row that had just been claimed and successfully sent
// seconds earlier (if the process crashed before MarkSent ran), causing the
// next dispatch tick to re-claim and re-send it — a real duplicate delivery
// to the recipient. claimed_at is only set by ClaimDue, so a 'pending' or
// terminal row is never matched by the claimed_at < olderThan condition
// regardless of how old it is. It returns the number of rows reset.
func (s *Store) ResetStale(olderThan time.Time) (int, error) {
	res, err := s.exec(`UPDATE scheduled_send SET status = ? WHERE status = ? AND claimed_at > 0 AND claimed_at < ?`,
		StatusPending, StatusSending, storage.Unix(olderThan))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}
