package apikey

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no cgo)
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

// Store is the SQLite-backed persistence for API keys. It reuses the same
// database file as the DAV store (opened separately, safe under WAL).
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path and applies the
// schema, mirroring the DAV store's WAL + single-connection setup.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS api_keys (
    id            TEXT NOT NULL PRIMARY KEY,
    token_sha256  TEXT NOT NULL,
    prefix        TEXT NOT NULL DEFAULT '',
    mailbox       TEXT NOT NULL,
    label         TEXT NOT NULL DEFAULT '',
    scopes        TEXT NOT NULL DEFAULT '',
    secret_enc    BLOB NOT NULL,
    secret_nonce  BLOB NOT NULL,
    mc_app_pw_id  TEXT NOT NULL DEFAULT '',
    created       INTEGER NOT NULL,
    last_used     INTEGER NOT NULL DEFAULT 0,
    expires       INTEGER NOT NULL DEFAULT 0,
    revoked       INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_api_keys_mailbox ON api_keys(mailbox);
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_token ON api_keys(token_sha256);`
	_, err := s.db.Exec(schema)
	return err
}

func unix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func fromUnix(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(n, 0).UTC()
}

// Create inserts a fully-formed record (including the recovered mailcow
// app-password id and the encrypted secret).
func (s *Store) Create(r Record) error {
	_, err := s.db.Exec(
		`INSERT INTO api_keys
            (id, token_sha256, prefix, mailbox, label, scopes, secret_enc, secret_nonce, mc_app_pw_id, created, last_used, expires, revoked)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.TokenSHA256, r.Prefix, r.Mailbox, r.Label, r.Scopes, r.SecretEnc, r.SecretNonce, r.MCAppPwID,
		unix(r.Created), unix(r.LastUsed), unix(r.Expires), unix(r.Revoked),
	)
	return err
}

// GetByID returns the full record (including secret material) for the auth path,
// or nil when no key with that id exists.
func (s *Store) GetByID(id string) (*Record, error) {
	var r Record
	var created, lastUsed, expires, revoked int64
	err := s.db.QueryRow(
		`SELECT id, token_sha256, prefix, mailbox, label, scopes, secret_enc, secret_nonce, mc_app_pw_id, created, last_used, expires, revoked
         FROM api_keys WHERE id = ?`, id).
		Scan(&r.ID, &r.TokenSHA256, &r.Prefix, &r.Mailbox, &r.Label, &r.Scopes, &r.SecretEnc, &r.SecretNonce, &r.MCAppPwID,
			&created, &lastUsed, &expires, &revoked)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Created = fromUnix(created)
	r.LastUsed = fromUnix(lastUsed)
	r.Expires = fromUnix(expires)
	r.Revoked = fromUnix(revoked)
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

	rows, err := s.db.Query(query, args...)
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
		r.Created = fromUnix(created)
		r.LastUsed = fromUnix(lastUsed)
		r.Expires = fromUnix(expires)
		r.Revoked = fromUnix(revoked)
		out = append(out, r)
	}
	return out, rows.Err()
}

// Revoke marks a key revoked at time t (idempotent — an already-revoked key
// keeps its original revocation time). It reports whether a row existed.
func (s *Store) Revoke(id string, t time.Time) (bool, error) {
	res, err := s.db.Exec(`UPDATE api_keys SET revoked = ? WHERE id = ? AND revoked = 0`, unix(t), id)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return true, nil
	}
	// No row updated: either it does not exist or it was already revoked.
	var exists int
	if err := s.db.QueryRow(`SELECT 1 FROM api_keys WHERE id = ?`, id).Scan(&exists); err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// TouchLastUsed records that a key was just used. Callers treat failures as
// non-fatal (it is a best-effort bookkeeping update).
func (s *Store) TouchLastUsed(id string, t time.Time) error {
	_, err := s.db.Exec(`UPDATE api_keys SET last_used = ? WHERE id = ?`, unix(t), id)
	return err
}
