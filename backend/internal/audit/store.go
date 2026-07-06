// Package audit records who did what, when, in Mailfold's own administration
// surface: the super-admin's and domain admins' logins (successful and
// failed) and every mutating action (create/update/delete) either of them
// takes. It deliberately does NOT cover regular webmail mailbox activity
// (sending mail, reading a folder, flagging a message) — that is normal
// mailbox usage, not administration, and logging it at this granularity would
// bury the signal that actually matters for a security review.
//
// It runs on the same database as the admin/DAV/API-key/webmail-user/
// domain-admin stores, following their exact Open/migrate/Dialect pattern.
package audit

import (
	"database/sql"
	"time"

	"github.com/isi1988/Mailfold/backend/storage"
)

// Entry is one recorded event.
type Entry struct {
	ID int64     `json:"id"`
	At time.Time `json:"at"`
	// Actor is the username (admin or domain-admin); empty for a failed
	// login where the identity is unconfirmed.
	Actor string `json:"actor"`
	// ActorType distinguishes which authentication tier Actor belongs to:
	// "admin" or "domain_admin".
	ActorType string `json:"actor_type"`
	// Action is a short, stable machine-readable label: "login",
	// "login_failed", "logout", or "METHOD /path" for a mutating request.
	Action string `json:"action"`
	Status int    `json:"status"`
	IP     string `json:"ip"`
}

// Store is the persistence layer for the audit log.
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
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
    id          ` + s.d.IntType() + ` PRIMARY KEY,
    at          ` + s.d.IntType() + ` NOT NULL,
    actor       TEXT NOT NULL DEFAULT '',
    actor_type  TEXT NOT NULL DEFAULT '',
    action      TEXT NOT NULL DEFAULT '',
    status      INTEGER NOT NULL DEFAULT 0,
    ip          TEXT NOT NULL DEFAULT ''
)`)
	return err
}

// Record appends one entry. It is best-effort by design (see the api package's
// call sites): a failure to write an audit row must never fail the request it
// describes.
func (s *Store) Record(e Entry) error {
	_, err := s.db.Exec(s.d.Rebind(`INSERT INTO audit_log (at, actor, actor_type, action, status, ip) VALUES (?, ?, ?, ?, ?, ?)`),
		storage.Unix(e.At), e.Actor, e.ActorType, e.Action, e.Status, e.IP)
	return err
}

// List returns up to limit entries starting at offset, newest first, together
// with the total row count (for pagination).
func (s *Store) List(limit, offset int) ([]Entry, int, error) {
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(s.d.Rebind(`SELECT id, at, actor, actor_type, action, status, ip FROM audit_log ORDER BY id DESC LIMIT ? OFFSET ?`), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var out []Entry
	for rows.Next() {
		var e Entry
		var at int64
		if err := rows.Scan(&e.ID, &at, &e.Actor, &e.ActorType, &e.Action, &e.Status, &e.IP); err != nil {
			return nil, 0, err
		}
		e.At = time.Unix(at, 0).UTC()
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
