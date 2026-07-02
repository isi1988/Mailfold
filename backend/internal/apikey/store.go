package apikey

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/storage"
)

// Record is one stored API key. The secret columns (TokenSHA256, SecretEnc,
// SecretNonce) are only ever read by the auth path; List never selects them.
type Record struct {
	ID          string // public key id (kid); primary key
	TokenSHA256 string // hex SHA-256 of the full token
	Prefix      string // display-only leading chars of the token
	Mailbox     string // owning mailbox address
	Label       string // human display name
	Scopes      string // comma-joined scope tokens
	SecretEnc   []byte // AES-256-GCM ciphertext of the mailcow app-password
	SecretNonce []byte // GCM nonce
	MCAppPwID   string // mailcow app-password id, for hard revoke
	Created     time.Time
	LastUsed    time.Time // zero = never used
	Expires     time.Time // zero = never expires
	Revoked     time.Time // zero = active
}

// Active reports whether the key is usable at time now (not revoked, not expired).
func (r *Record) Active(now time.Time) bool {
	if !r.Revoked.IsZero() {
		return false
	}
	if !r.Expires.IsZero() && !now.Before(r.Expires) {
		return false
	}
	return true
}

// Store is the persistence layer for API keys. It runs on any database the
// storage package has a driver for (SQLite in the open-source build, PostgreSQL
// in the enterprise build); all SQL is written once and adapted through the
// dialect.
type Store struct {
	db *sql.DB
	d  storage.Dialect
}

// Open opens the API-key database on the given driver and DSN and applies the
// schema. It reuses the same file/instance as the DAV store.
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
	schema := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS api_keys (
    id            TEXT NOT NULL PRIMARY KEY,
    token_sha256  TEXT NOT NULL,
    prefix        TEXT NOT NULL DEFAULT '',
    mailbox       TEXT NOT NULL,
    label         TEXT NOT NULL DEFAULT '',
    scopes        TEXT NOT NULL DEFAULT '',
    secret_enc    %[1]s NOT NULL,
    secret_nonce  %[1]s NOT NULL,
    mc_app_pw_id  TEXT NOT NULL DEFAULT '',
    created       %[2]s NOT NULL,
    last_used     %[2]s NOT NULL DEFAULT 0,
    expires       %[2]s NOT NULL DEFAULT 0,
    revoked       %[2]s NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_api_keys_mailbox ON api_keys(mailbox);
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_token ON api_keys(token_sha256);`,
		s.d.BlobType(), s.d.IntType())
	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) exec(query string, args ...any) (sql.Result, error) {
	return s.db.Exec(s.d.Rebind(query), args...)
}

// Create inserts a fully-formed record (including the recovered mailcow
// app-password id and the encrypted secret).
func (s *Store) Create(r Record) error {
	_, err := s.exec(
		`INSERT INTO api_keys
            (id, token_sha256, prefix, mailbox, label, scopes, secret_enc, secret_nonce, mc_app_pw_id, created, last_used, expires, revoked)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.TokenSHA256, r.Prefix, r.Mailbox, r.Label, r.Scopes, r.SecretEnc, r.SecretNonce, r.MCAppPwID,
		storage.Unix(r.Created), storage.Unix(r.LastUsed), storage.Unix(r.Expires), storage.Unix(r.Revoked),
	)
	return err
}

// GetByID returns the full record (including secret material) for the auth path,
// or nil when no key with that id exists.
func (s *Store) GetByID(id string) (*Record, error) {
	var r Record
	var created, lastUsed, expires, revoked int64
	err := s.db.QueryRow(s.d.Rebind(
		`SELECT id, token_sha256, prefix, mailbox, label, scopes, secret_enc, secret_nonce, mc_app_pw_id, created, last_used, expires, revoked
         FROM api_keys WHERE id = ?`), id).
		Scan(&r.ID, &r.TokenSHA256, &r.Prefix, &r.Mailbox, &r.Label, &r.Scopes, &r.SecretEnc, &r.SecretNonce, &r.MCAppPwID,
			&created, &lastUsed, &expires, &revoked)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Created = storage.FromUnix(created)
	r.LastUsed = storage.FromUnix(lastUsed)
	r.Expires = storage.FromUnix(expires)
	r.Revoked = storage.FromUnix(revoked)
	return &r, nil
}

// List returns key metadata, never selecting the secret columns. An empty
// mailbox filter returns every key.
func (s *Store) List(mailbox string) ([]Record, error) {
	query := `SELECT id, prefix, mailbox, label, scopes, mc_app_pw_id, created, last_used, expires, revoked FROM api_keys`
	args := []any{}
	if mailbox != "" {
		query += ` WHERE mailbox = ?`
		args = append(args, mailbox)
	}
	query += ` ORDER BY created DESC`

	rows, err := s.db.Query(s.d.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []Record
	for rows.Next() {
		var r Record
		var created, lastUsed, expires, revoked int64
		if err := rows.Scan(&r.ID, &r.Prefix, &r.Mailbox, &r.Label, &r.Scopes, &r.MCAppPwID,
			&created, &lastUsed, &expires, &revoked); err != nil {
			return nil, err
		}
		r.Created = storage.FromUnix(created)
		r.LastUsed = storage.FromUnix(lastUsed)
		r.Expires = storage.FromUnix(expires)
		r.Revoked = storage.FromUnix(revoked)
		out = append(out, r)
	}
	return out, rows.Err()
}

// Revoke marks a key revoked at time t (idempotent — an already-revoked key
// keeps its original revocation time). It reports whether a row existed.
func (s *Store) Revoke(id string, t time.Time) (bool, error) {
	res, err := s.exec(`UPDATE api_keys SET revoked = ? WHERE id = ? AND revoked = 0`, storage.Unix(t), id)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return true, nil
	}
	// No row updated: either it does not exist or it was already revoked.
	var exists int
	if err := s.db.QueryRow(s.d.Rebind(`SELECT 1 FROM api_keys WHERE id = ?`), id).Scan(&exists); err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// TouchLastUsed records that a key was just used. Callers treat failures as
// non-fatal (it is a best-effort bookkeeping update).
func (s *Store) TouchLastUsed(id string, t time.Time) error {
	_, err := s.exec(`UPDATE api_keys SET last_used = ? WHERE id = ?`, storage.Unix(t), id)
	return err
}
