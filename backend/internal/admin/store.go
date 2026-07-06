// Package admin persists the single administrator account's mutable state:
// an optional password-hash override (so the password can be changed without
// restarting the process with a new MAILFOLD_ADMIN_PASSWORD), profile fields,
// two-factor (TOTP) enrollment, recovery codes, the system-notification sender
// mailbox, and password-reset tokens. It runs on the same SQLite database as
// the DAV and API-key stores (backend/internal/dav, backend/internal/apikey),
// following their exact Open/migrate/Dialect pattern.
package admin

import (
	"database/sql"
	"time"

	"github.com/isi1988/Mailfold/backend/storage"
)

// Account is the admin account's stored, mutable state. Secret fields
// (PasswordHash, TOTP/notify ciphertext) are only read by the auth and
// account-settings paths, never returned to the frontend verbatim.
type Account struct {
	Username string

	// PasswordHash is a bcrypt hash overriding the configured
	// MAILFOLD_ADMIN_PASSWORD once the admin has changed their password. Empty
	// means "no override yet — compare against the configured password".
	PasswordHash string

	DisplayName string
	Email       string
	Timezone    string
	AvatarURL   string

	TOTPEnabled     bool
	TOTPSecretEnc   []byte
	TOTPSecretNonce []byte

	// NotifyMailbox/NotifyPasswordEnc are the mailbox Mailfold authenticates as
	// (over the same SMTP server webmail already uses) to send system emails
	// such as a password-reset link. Empty NotifyMailbox means unconfigured.
	NotifyMailbox       string
	NotifyPasswordEnc   []byte
	NotifyPasswordNonce []byte

	UpdatedAt time.Time
}

// ResetToken is a single-use, time-limited password-reset token. Only its
// SHA-256 hash is stored; the raw token exists solely in the emailed link.
type ResetToken struct {
	TokenHash string
	Username  string
	ExpiresAt time.Time
	UsedAt    time.Time // zero = unused
}

// Store is the persistence layer for the admin account.
type Store struct {
	db *sql.DB
	d  storage.Dialect
}

// Open opens the admin database on the given driver and DSN and applies the
// schema. It reuses the same file/instance as the DAV and API-key stores.
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
		`CREATE TABLE IF NOT EXISTS admin_account (
    username              TEXT NOT NULL PRIMARY KEY,
    password_hash         TEXT NOT NULL DEFAULT '',
    display_name          TEXT NOT NULL DEFAULT '',
    email                 TEXT NOT NULL DEFAULT '',
    timezone               TEXT NOT NULL DEFAULT '',
    avatar_url             TEXT NOT NULL DEFAULT '',
    totp_enabled            INTEGER NOT NULL DEFAULT 0,
    totp_secret_enc         ` + s.d.BlobType() + `,
    totp_secret_nonce       ` + s.d.BlobType() + `,
    notify_mailbox          TEXT NOT NULL DEFAULT '',
    notify_password_enc     ` + s.d.BlobType() + `,
    notify_password_nonce   ` + s.d.BlobType() + `,
    updated_at              ` + s.d.IntType() + ` NOT NULL DEFAULT 0
)`,
		`CREATE TABLE IF NOT EXISTS admin_recovery_code (
    username   TEXT NOT NULL,
    code_hash  TEXT NOT NULL,
    used_at    ` + s.d.IntType() + ` NOT NULL DEFAULT 0,
    PRIMARY KEY (username, code_hash)
)`,
		`CREATE TABLE IF NOT EXISTS admin_reset_token (
    token_hash  TEXT NOT NULL PRIMARY KEY,
    username    TEXT NOT NULL,
    expires_at  ` + s.d.IntType() + ` NOT NULL,
    used_at     ` + s.d.IntType() + ` NOT NULL DEFAULT 0
)`,
		`CREATE TABLE IF NOT EXISTS admin_known_device (
    username     TEXT NOT NULL,
    fingerprint  TEXT NOT NULL,
    first_seen   ` + s.d.IntType() + ` NOT NULL,
    last_seen    ` + s.d.IntType() + ` NOT NULL,
    PRIMARY KEY (username, fingerprint)
)`,
		`CREATE TABLE IF NOT EXISTS admin_webauthn_credential (
    id             ` + s.d.IntType() + ` PRIMARY KEY,
    username       TEXT NOT NULL,
    credential_id  ` + s.d.BlobType() + ` NOT NULL,
    public_key     ` + s.d.BlobType() + ` NOT NULL,
    sign_count     INTEGER NOT NULL DEFAULT 0,
    transports     TEXT NOT NULL DEFAULT '',
    name           TEXT NOT NULL DEFAULT '',
    created_at     ` + s.d.IntType() + ` NOT NULL,
    UNIQUE (credential_id)
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

// GetAccount returns the stored row for username, or a zero-value Account
// (Username still set) when the admin has never changed anything yet — the
// row is created lazily on first write, not on read.
func (s *Store) GetAccount(username string) (Account, error) {
	a := Account{Username: username}
	var totpEnabled int
	var updatedAt int64
	err := s.db.QueryRow(s.d.Rebind(`
        SELECT password_hash, display_name, email, timezone, avatar_url,
               totp_enabled, totp_secret_enc, totp_secret_nonce,
               notify_mailbox, notify_password_enc, notify_password_nonce, updated_at
        FROM admin_account WHERE username = ?`), username).Scan(
		&a.PasswordHash, &a.DisplayName, &a.Email, &a.Timezone, &a.AvatarURL,
		&totpEnabled, &a.TOTPSecretEnc, &a.TOTPSecretNonce,
		&a.NotifyMailbox, &a.NotifyPasswordEnc, &a.NotifyPasswordNonce, &updatedAt,
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
// partial UPDATEs (password only, profile only, …) have a row to affect.
func (s *Store) ensureRow(username string) error {
	_, err := s.exec(`INSERT INTO admin_account (username) VALUES (?)
        ON CONFLICT(username) DO NOTHING`, username)
	if err != nil {
		// SQLite driver here supports ON CONFLICT; if a future dialect does not,
		// fall back to an existence check.
		var exists int
		checkErr := s.db.QueryRow(s.d.Rebind(`SELECT 1 FROM admin_account WHERE username = ?`), username).Scan(&exists)
		if checkErr == nil {
			return nil
		}
		if checkErr == sql.ErrNoRows {
			_, err2 := s.exec(`INSERT INTO admin_account (username) VALUES (?)`, username)
			return err2
		}
		return err
	}
	return nil
}

// SetPasswordHash stores (or replaces) the admin's password-hash override.
func (s *Store) SetPasswordHash(username, hash string, now time.Time) error {
	if err := s.ensureRow(username); err != nil {
		return err
	}
	_, err := s.exec(`UPDATE admin_account SET password_hash = ?, updated_at = ? WHERE username = ?`,
		hash, storage.Unix(now), username)
	return err
}

// ProfileUpdate is the set of editable profile fields. It is decoded directly
// from the PUT /api/account/profile request body, so its tags are the wire
// contract.
type ProfileUpdate struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Timezone    string `json:"timezone"`
	AvatarURL   string `json:"avatar_url"`
}

// SetProfile persists the admin's profile fields.
func (s *Store) SetProfile(username string, p ProfileUpdate, now time.Time) error {
	if err := s.ensureRow(username); err != nil {
		return err
	}
	_, err := s.exec(`UPDATE admin_account SET display_name = ?, email = ?, timezone = ?, avatar_url = ?, updated_at = ? WHERE username = ?`,
		p.DisplayName, p.Email, p.Timezone, p.AvatarURL, storage.Unix(now), username)
	return err
}

// SetTOTP stores the (encrypted) TOTP secret and enrollment state.
func (s *Store) SetTOTP(username string, enabled bool, secretEnc, secretNonce []byte, now time.Time) error {
	if err := s.ensureRow(username); err != nil {
		return err
	}
	enc := 0
	if enabled {
		enc = 1
	}
	_, err := s.exec(`UPDATE admin_account SET totp_enabled = ?, totp_secret_enc = ?, totp_secret_nonce = ?, updated_at = ? WHERE username = ?`,
		enc, secretEnc, secretNonce, storage.Unix(now), username)
	return err
}

// SetNotifySender stores the (encrypted) system-notification sender mailbox.
// An empty mailbox clears the configuration.
func (s *Store) SetNotifySender(username, mailbox string, passwordEnc, passwordNonce []byte, now time.Time) error {
	if err := s.ensureRow(username); err != nil {
		return err
	}
	_, err := s.exec(`UPDATE admin_account SET notify_mailbox = ?, notify_password_enc = ?, notify_password_nonce = ?, updated_at = ? WHERE username = ?`,
		mailbox, passwordEnc, passwordNonce, storage.Unix(now), username)
	return err
}

// ReplaceRecoveryCodes deletes any existing recovery codes for username and
// stores the new set (already hashed by the caller via HashRecoveryCode).
func (s *Store) ReplaceRecoveryCodes(username string, hashes []string) error {
	if _, err := s.exec(`DELETE FROM admin_recovery_code WHERE username = ?`, username); err != nil {
		return err
	}
	for _, h := range hashes {
		if _, err := s.exec(`INSERT INTO admin_recovery_code (username, code_hash) VALUES (?, ?)`, username, h); err != nil {
			return err
		}
	}
	return nil
}

// ConsumeRecoveryCode marks a matching, unused recovery code as used and
// reports whether one was found. Codes are looked up by their hash (computed
// by the caller via HashRecoveryCode), so this is a direct, constant-cost
// lookup rather than a linear scan.
func (s *Store) ConsumeRecoveryCode(username, codeHash string, now time.Time) (bool, error) {
	res, err := s.exec(`UPDATE admin_recovery_code SET used_at = ? WHERE username = ? AND code_hash = ? AND used_at = 0`,
		storage.Unix(now), username, codeHash)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// RemainingRecoveryCodes counts how many unused recovery codes username has.
func (s *Store) RemainingRecoveryCodes(username string) (int, error) {
	var n int
	err := s.db.QueryRow(s.d.Rebind(`SELECT COUNT(*) FROM admin_recovery_code WHERE username = ? AND used_at = 0`), username).Scan(&n)
	return n, err
}

// CreateResetToken stores a password-reset token's hash.
func (s *Store) CreateResetToken(tokenHash, username string, expiresAt time.Time) error {
	_, err := s.exec(`INSERT INTO admin_reset_token (token_hash, username, expires_at) VALUES (?, ?, ?)`,
		tokenHash, username, storage.Unix(expiresAt))
	return err
}

// ConsumeResetToken marks a reset token used and returns the username it was
// issued for, only if it exists, is unused, and has not expired.
func (s *Store) ConsumeResetToken(tokenHash string, now time.Time) (string, bool, error) {
	var username string
	var expiresAt, usedAt int64
	err := s.db.QueryRow(s.d.Rebind(`SELECT username, expires_at, used_at FROM admin_reset_token WHERE token_hash = ?`), tokenHash).
		Scan(&username, &expiresAt, &usedAt)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if usedAt != 0 || now.After(storage.FromUnix(expiresAt)) {
		return "", false, nil
	}
	if _, err := s.exec(`UPDATE admin_reset_token SET used_at = ? WHERE token_hash = ?`, storage.Unix(now), tokenHash); err != nil {
		return "", false, err
	}
	return username, true, nil
}

// IsKnownDevice reports whether fingerprint has signed in as username before.
func (s *Store) IsKnownDevice(username, fingerprint string) (bool, error) {
	var n int
	err := s.db.QueryRow(s.d.Rebind(`SELECT COUNT(*) FROM admin_known_device WHERE username = ? AND fingerprint = ?`), username, fingerprint).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// RecordDevice records (or refreshes the last-seen time of) a device
// fingerprint for username, so a later sign-in from the same device is not
// treated as new.
func (s *Store) RecordDevice(username, fingerprint string, now time.Time) error {
	_, err := s.exec(`INSERT INTO admin_known_device (username, fingerprint, first_seen, last_seen) VALUES (?, ?, ?, ?)
        ON CONFLICT(username, fingerprint) DO UPDATE SET last_seen = excluded.last_seen`,
		username, fingerprint, storage.Unix(now), storage.Unix(now))
	return err
}

// WebAuthnCredential is one enrolled security key or passkey for the admin
// account. CredentialID/PublicKey/SignCount/Transports round-trip through the
// go-webauthn library unchanged; Name is a caller-chosen label (e.g. "MacBook
// Touch ID") shown in Settings so multiple credentials can be told apart.
type WebAuthnCredential struct {
	ID           int64
	Username     string
	CredentialID []byte
	PublicKey    []byte
	SignCount    uint32
	// Transports is a comma-joined list of the authenticator's reported
	// transports (e.g. "internal,hybrid"); empty when the authenticator
	// didn't report any.
	Transports string
	Name       string
	CreatedAt  time.Time
}

// ListWebAuthnCredentials returns every credential enrolled for username,
// oldest first.
func (s *Store) ListWebAuthnCredentials(username string) ([]WebAuthnCredential, error) {
	rows, err := s.db.Query(s.d.Rebind(`SELECT id, credential_id, public_key, sign_count, transports, name, created_at
        FROM admin_webauthn_credential WHERE username = ? ORDER BY id`), username)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []WebAuthnCredential
	for rows.Next() {
		c := WebAuthnCredential{Username: username}
		var createdAt int64
		if err := rows.Scan(&c.ID, &c.CredentialID, &c.PublicKey, &c.SignCount, &c.Transports, &c.Name, &createdAt); err != nil {
			return nil, err
		}
		c.CreatedAt = storage.FromUnix(createdAt)
		out = append(out, c)
	}
	return out, rows.Err()
}

// AddWebAuthnCredential stores a newly-registered credential.
func (s *Store) AddWebAuthnCredential(c WebAuthnCredential, now time.Time) error {
	_, err := s.exec(`INSERT INTO admin_webauthn_credential (username, credential_id, public_key, sign_count, transports, name, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.Username, c.CredentialID, c.PublicKey, c.SignCount, c.Transports, c.Name, storage.Unix(now))
	return err
}

// DeleteWebAuthnCredential revokes one of username's credentials by its
// database id, scoped to username so one admin can never delete another
// account's credential (moot today with a single admin, but cheap to get
// right).
func (s *Store) DeleteWebAuthnCredential(username string, id int64) error {
	_, err := s.exec(`DELETE FROM admin_webauthn_credential WHERE username = ? AND id = ?`, username, id)
	return err
}

// UpdateWebAuthnSignCount persists the authenticator's signature counter
// after a successful login, so a future clone-detection comparison has an
// up-to-date baseline.
func (s *Store) UpdateWebAuthnSignCount(credentialID []byte, count uint32) error {
	_, err := s.exec(`UPDATE admin_webauthn_credential SET sign_count = ? WHERE credential_id = ?`, count, credentialID)
	return err
}
