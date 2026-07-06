// Package sessionstore is a durable, optional backing store for Mailfold's
// three bearer-token session managers (admin, webmail, domain-admin) and
// their pending-2FA intermediate state.
//
// Every session manager in this codebase (internal/auth.Authenticator,
// internal/webmail.Sessions, internal/domainadmin.Sessions) was originally an
// in-process map: fine for a single node, but it means a bearer token minted
// by one backend instance is invisible to any other — which rules out
// horizontal scaling behind a load balancer without sticky sessions. This
// package gives each manager a shared, durable place to keep that state
// instead, using the exact same SQLite/Postgres database already used for the
// DAV, API-key, admin, audit, and WebAuthn stores.
//
// It is optional and additive: a session manager attaches a Store only when
// MAILFOLD_DB_PATH is configured (see each manager's SetStore-style method);
// without one, every manager falls back to its original in-memory map,
// exactly like every other optional Mailfold feature. A manager that needs to
// hold a secret alongside a session (webmail's mailbox password, required on
// every IMAP/SMTP call) additionally needs a Cipher, built from
// MAILFOLD_ADMIN_ENC_KEY like every other at-rest secret in this codebase;
// without one, that manager's sessions specifically stay in-memory even with
// a database configured, rather than persisting a secret in the clear.
package sessionstore

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"

	"github.com/isi1988/Mailfold/backend/storage"
)

// Row is one persisted token: its owner, an optional encrypted secret, an
// opaque metadata blob (each manager defines its own JSON shape), and the
// pending-2FA retry counter.
type Row struct {
	Subject     string
	Secret      []byte
	SecretNonce []byte
	Meta        string
	Attempts    int
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// Store is the persistence layer shared by every session manager. Rows are
// scoped by "kind" (e.g. "admin_session", "admin_pending", "webmail_session")
// so one physical table backs all of them without their tokens colliding.
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
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS session_token (
    token_hash    TEXT NOT NULL,
    kind          TEXT NOT NULL,
    subject       TEXT NOT NULL DEFAULT '',
    secret_enc    ` + s.d.BlobType() + `,
    secret_nonce  ` + s.d.BlobType() + `,
    meta          TEXT NOT NULL DEFAULT '',
    attempts      INTEGER NOT NULL DEFAULT 0,
    created_at    ` + s.d.IntType() + ` NOT NULL,
    expires_at    ` + s.d.IntType() + ` NOT NULL,
    PRIMARY KEY (token_hash, kind)
)`)
	return err
}

// hashToken derives the storage key for a raw bearer token. Only the hash is
// ever persisted, matching the convention already used for password-reset and
// API-key tokens elsewhere in this codebase — a stolen database backup or a
// read-only SQL bug can never leak a usable session token.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// PutParams is the full set of fields for a new (or replaced) session row,
// grouped into a struct because Put would otherwise need eight positional
// parameters.
type PutParams struct {
	Token   string
	Kind    string
	Subject string
	// Secret/SecretNonce may both be nil when the manager has nothing to
	// encrypt (e.g. admin and domain-admin sessions, unlike webmail's).
	Secret      []byte
	SecretNonce []byte
	Meta        string
	Now         time.Time
	ExpiresAt   time.Time
}

// Put inserts (or replaces, if the same token+kind is reused) a session row.
func (s *Store) Put(p PutParams) error {
	_, err := s.db.Exec(s.d.Rebind(`INSERT INTO session_token (token_hash, kind, subject, secret_enc, secret_nonce, meta, attempts, created_at, expires_at)
        VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)
        ON CONFLICT(token_hash, kind) DO UPDATE SET subject = excluded.subject, secret_enc = excluded.secret_enc,
            secret_nonce = excluded.secret_nonce, meta = excluded.meta, attempts = 0, created_at = excluded.created_at, expires_at = excluded.expires_at`),
		hashToken(p.Token), p.Kind, p.Subject, p.Secret, p.SecretNonce, p.Meta, storage.Unix(p.Now), storage.Unix(p.ExpiresAt))
	return err
}

// Get returns the row for token+kind if present and not already expired,
// deleting it (and reporting ok=false) if its expiry has passed.
func (s *Store) Get(token, kind string, now time.Time) (Row, bool, error) {
	var row Row
	var createdAt, expiresAt int64
	err := s.db.QueryRow(s.d.Rebind(`SELECT subject, secret_enc, secret_nonce, meta, attempts, created_at, expires_at
        FROM session_token WHERE token_hash = ? AND kind = ?`), hashToken(token), kind).
		Scan(&row.Subject, &row.Secret, &row.SecretNonce, &row.Meta, &row.Attempts, &createdAt, &expiresAt)
	if err == sql.ErrNoRows {
		return Row{}, false, nil
	}
	if err != nil {
		return Row{}, false, err
	}
	row.CreatedAt = storage.FromUnix(createdAt)
	row.ExpiresAt = storage.FromUnix(expiresAt)
	if now.After(row.ExpiresAt) {
		_ = s.Delete(token, kind)
		return Row{}, false, nil
	}
	return row, true, nil
}

// IncrementAttempts atomically bumps token+kind's retry counter and returns
// the post-increment value together with the row's subject (so a caller
// doesn't need a second round trip to learn who the token belongs to), so a
// pending 2FA token can be retried a bounded number of times without a
// check-then-act race — across concurrent requests, and (unlike an
// in-process map) across multiple backend instances sharing this database.
// Reports ok=false if the token is unknown or already expired (deleting it
// in the latter case).
func (s *Store) IncrementAttempts(token, kind string, now time.Time) (attempts int, subject string, ok bool, err error) {
	var expiresAt int64
	err = s.db.QueryRow(s.d.Rebind(`UPDATE session_token SET attempts = attempts + 1
        WHERE token_hash = ? AND kind = ? RETURNING attempts, subject, expires_at`), hashToken(token), kind).
		Scan(&attempts, &subject, &expiresAt)
	if err == sql.ErrNoRows {
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, err
	}
	if now.After(storage.FromUnix(expiresAt)) {
		_ = s.Delete(token, kind)
		return 0, "", false, nil
	}
	return attempts, subject, true, nil
}

// Delete removes one token+kind row unconditionally. Deleting an unknown
// token is a harmless no-op, so logout/consume can call it without checking
// existence first.
func (s *Store) Delete(token, kind string) error {
	_, err := s.db.Exec(s.d.Rebind(`DELETE FROM session_token WHERE token_hash = ? AND kind = ?`), hashToken(token), kind)
	return err
}

// ListBySubject returns every non-expired row of kind belonging to subject,
// each tagged with the token hash it was stored under (so a caller can
// derive the same stable, non-reversible session ID the in-memory managers
// already expose, without ever handling the raw token).
func (s *Store) ListBySubject(kind, subject string, now time.Time) ([]ListedRow, error) {
	rows, err := s.db.Query(s.d.Rebind(`SELECT token_hash, meta, attempts, created_at, expires_at
        FROM session_token WHERE kind = ? AND subject = ? AND expires_at > ?`), kind, subject, storage.Unix(now))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []ListedRow
	for rows.Next() {
		var lr ListedRow
		var createdAt, expiresAt int64
		if err := rows.Scan(&lr.TokenHash, &lr.Meta, &lr.Attempts, &createdAt, &expiresAt); err != nil {
			return nil, err
		}
		lr.CreatedAt = storage.FromUnix(createdAt)
		lr.ExpiresAt = storage.FromUnix(expiresAt)
		out = append(out, lr)
	}
	return out, rows.Err()
}

// ListedRow is one row returned by ListBySubject — like Row, but identified by
// its token hash (for building a session-list UI) rather than requiring the
// caller to already hold the raw token.
type ListedRow struct {
	TokenHash string
	Meta      string
	Attempts  int
	CreatedAt time.Time
	ExpiresAt time.Time
}

// DeleteByHash removes a row by its stored hash directly (rather than by
// re-hashing a raw token), for revoking one entry out of a ListBySubject
// result. It reports whether a row was actually removed.
func (s *Store) DeleteByHash(tokenHash, kind string) (bool, error) {
	res, err := s.db.Exec(s.d.Rebind(`DELETE FROM session_token WHERE token_hash = ? AND kind = ?`), tokenHash, kind)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// DeleteBySubjectExcept removes every row of kind for subject except the one
// matching keepToken (pass "" to remove all of them), returning how many were
// deleted. It backs a "sign out all other devices" action.
func (s *Store) DeleteBySubjectExcept(kind, subject, keepToken string) (int, error) {
	res, err := s.db.Exec(s.d.Rebind(`DELETE FROM session_token WHERE kind = ? AND subject = ? AND token_hash != ?`),
		kind, subject, hashToken(keepToken))
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// GC deletes every row (of any kind) whose expiry has already passed. It is
// intended to be called periodically from a background goroutine, exactly
// like every in-memory manager's own GC method.
func (s *Store) GC(now time.Time) error {
	_, err := s.db.Exec(s.d.Rebind(`DELETE FROM session_token WHERE expires_at <= ?`), storage.Unix(now))
	return err
}
