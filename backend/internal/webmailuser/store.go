// Package webmailuser persists per-mailbox state for webmail users — a
// signature and optional two-factor (TOTP) enrollment — as opposed to
// internal/admin, which persists the single configured Mailfold
// administrator's account. It runs on the same database as the admin,
// DAV, and API-key stores (backend/internal/dav, backend/internal/apikey),
// following their exact Open/migrate/Dialect pattern. The TOTP/recovery-code
// cryptographic primitives themselves are not duplicated here — they are the
// already-exported, principal-agnostic helpers in internal/admin
// (NewTOTPSecret, TOTPURI, VerifyTOTP, NewRecoveryCodes, HashRecoveryCode).
package webmailuser

import (
	"database/sql"
	"time"

	"github.com/isi1988/Mailfold/backend/storage"
)

// Account is one mailbox user's stored, mutable webmail state, keyed by their
// mailbox address (not their Mailfold session — a mailbox has exactly one
// account row regardless of how many admins or devices have opened it).
type Account struct {
	Email string

	Signature string

	TOTPEnabled     bool
	TOTPSecretEnc   []byte
	TOTPSecretNonce []byte

	UpdatedAt time.Time
}

// Store is the persistence layer for webmail users.
type Store struct {
	db *sql.DB
	d  storage.Dialect
}

// Open opens the webmail-user database on the given driver and DSN and
// applies the schema. It reuses the same file/instance as the other stores.
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
		`CREATE TABLE IF NOT EXISTS webmail_user_account (
    email                 TEXT NOT NULL PRIMARY KEY,
    signature             TEXT NOT NULL DEFAULT '',
    totp_enabled          INTEGER NOT NULL DEFAULT 0,
    totp_secret_enc       ` + s.d.BlobType() + `,
    totp_secret_nonce     ` + s.d.BlobType() + `,
    updated_at            ` + s.d.IntType() + ` NOT NULL DEFAULT 0
)`,
		`CREATE TABLE IF NOT EXISTS webmail_user_recovery_code (
    email      TEXT NOT NULL,
    code_hash  TEXT NOT NULL,
    used_at    ` + s.d.IntType() + ` NOT NULL DEFAULT 0,
    PRIMARY KEY (email, code_hash)
)`,
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

// GetAccount returns the stored row for email, or a zero-value Account (Email
// still set) when the user has never changed anything yet — the row is
// created lazily on first write, not on read.
func (s *Store) GetAccount(email string) (Account, error) {
	a := Account{Email: email}
	var totpEnabled int
	var updatedAt int64
	err := s.db.QueryRow(s.d.Rebind(`
        SELECT signature, totp_enabled, totp_secret_enc, totp_secret_nonce, updated_at
        FROM webmail_user_account WHERE email = ?`), email).Scan(
		&a.Signature, &totpEnabled, &a.TOTPSecretEnc, &a.TOTPSecretNonce, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return a, nil
	}
	if err != nil {
		return Account{}, err
	}
	a.TOTPEnabled = totpEnabled != 0
	a.UpdatedAt = storage.FromUnix(updatedAt)
	return a, nil
}

// ensureRow creates the account row if it does not exist yet, so subsequent
// partial UPDATEs (signature only, TOTP only, …) have a row to affect.
func (s *Store) ensureRow(email string) error {
	_, err := s.exec(`INSERT INTO webmail_user_account (email) VALUES (?)
        ON CONFLICT(email) DO NOTHING`, email)
	if err != nil {
		var exists int
		checkErr := s.db.QueryRow(s.d.Rebind(`SELECT 1 FROM webmail_user_account WHERE email = ?`), email).Scan(&exists)
		if checkErr == nil {
			return nil
		}
		if checkErr == sql.ErrNoRows {
			_, err2 := s.exec(`INSERT INTO webmail_user_account (email) VALUES (?)`, email)
			return err2
		}
		return err
	}
	return nil
}

// SetSignature stores (or replaces) email's signature.
func (s *Store) SetSignature(email, signature string, now time.Time) error {
	if err := s.ensureRow(email); err != nil {
		return err
	}
	_, err := s.exec(`UPDATE webmail_user_account SET signature = ?, updated_at = ? WHERE email = ?`,
		signature, storage.Unix(now), email)
	return err
}

// SetTOTP stores the (encrypted) TOTP secret and enrollment state.
func (s *Store) SetTOTP(email string, enabled bool, secretEnc, secretNonce []byte, now time.Time) error {
	if err := s.ensureRow(email); err != nil {
		return err
	}
	enc := 0
	if enabled {
		enc = 1
	}
	_, err := s.exec(`UPDATE webmail_user_account SET totp_enabled = ?, totp_secret_enc = ?, totp_secret_nonce = ?, updated_at = ? WHERE email = ?`,
		enc, secretEnc, secretNonce, storage.Unix(now), email)
	return err
}

// ReplaceRecoveryCodes deletes any existing recovery codes for email and
// stores the new set (already hashed by the caller via admin.HashRecoveryCode).
func (s *Store) ReplaceRecoveryCodes(email string, hashes []string) error {
	if _, err := s.exec(`DELETE FROM webmail_user_recovery_code WHERE email = ?`, email); err != nil {
		return err
	}
	for _, h := range hashes {
		if _, err := s.exec(`INSERT INTO webmail_user_recovery_code (email, code_hash) VALUES (?, ?)`, email, h); err != nil {
			return err
		}
	}
	return nil
}

// ConsumeRecoveryCode marks a matching, unused recovery code as used and
// reports whether one was found.
func (s *Store) ConsumeRecoveryCode(email, codeHash string, now time.Time) (bool, error) {
	res, err := s.exec(`UPDATE webmail_user_recovery_code SET used_at = ? WHERE email = ? AND code_hash = ? AND used_at = 0`,
		storage.Unix(now), email, codeHash)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// RemainingRecoveryCodes counts how many unused recovery codes email has.
func (s *Store) RemainingRecoveryCodes(email string) (int, error) {
	var n int
	err := s.db.QueryRow(s.d.Rebind(`SELECT COUNT(*) FROM webmail_user_recovery_code WHERE email = ? AND used_at = 0`), email).Scan(&n)
	return n, err
}
