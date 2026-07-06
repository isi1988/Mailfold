// Package webpush persists Web Push subscriptions for webmail mailboxes (so
// a browser can be notified of new mail even with no tab open) and the
// server's own VAPID key pair, which identifies Mailfold as the sender to
// every push service. It follows the same Open/migrate/Dialect pattern as
// every other Mailfold store (internal/webmailuser, internal/apikey, ...),
// running on the same SQLite/Postgres database.
package webpush

import (
	"database/sql"
	"time"

	"github.com/isi1988/Mailfold/backend/storage"
)

// Subscription is one browser's push registration for a mailbox. A mailbox
// can have several (one per device/browser); each is polled independently.
// PasswordEnc/PasswordNonce hold the mailbox's own IMAP password, encrypted
// at rest — required so the background poller (internal/api) can check for
// new mail on this subscription's behalf even when no browser tab is open at
// all, exactly the point of a push notification. It is captured once, at
// subscribe time, from the caller's already-authenticated webmail session —
// the browser is never asked to type it again.
type Subscription struct {
	ID            int64
	Email         string
	Endpoint      string
	P256dh        string
	Auth          string
	PasswordEnc   []byte
	PasswordNonce []byte
	LastUID       uint32
	CreatedAt     time.Time
}

// Store is the persistence layer for Web Push subscriptions and the VAPID
// key pair.
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
		`CREATE TABLE IF NOT EXISTS webpush_vapid_key (
    id           INTEGER PRIMARY KEY,
    public_key   TEXT NOT NULL,
    private_key  TEXT NOT NULL
)`,
		`CREATE TABLE IF NOT EXISTS webpush_subscription (
    id              ` + s.d.IntType() + ` PRIMARY KEY,
    email           TEXT NOT NULL,
    endpoint        TEXT NOT NULL,
    p256dh          TEXT NOT NULL,
    auth            TEXT NOT NULL,
    password_enc    ` + s.d.BlobType() + ` NOT NULL,
    password_nonce  ` + s.d.BlobType() + ` NOT NULL,
    last_uid        INTEGER NOT NULL DEFAULT 0,
    created_at      ` + s.d.IntType() + ` NOT NULL,
    UNIQUE (endpoint)
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

// GetOrCreateVAPIDKeys returns the server's VAPID key pair, generating and
// persisting one on first use. Every Mailfold instance sharing this database
// ends up with the same key pair, which is required — every subscription a
// browser holds is bound to the public key it subscribed with, so the pair
// must stay stable for the life of the database.
func (s *Store) GetOrCreateVAPIDKeys(generate func() (private, public string, err error)) (public, private string, err error) {
	err = s.db.QueryRow(`SELECT public_key, private_key FROM webpush_vapid_key WHERE id = 1`).Scan(&public, &private)
	if err == nil {
		return public, private, nil
	}
	if err != sql.ErrNoRows {
		return "", "", err
	}
	private, public, err = generate()
	if err != nil {
		return "", "", err
	}
	_, err = s.exec(`INSERT INTO webpush_vapid_key (id, public_key, private_key) VALUES (1, ?, ?)
        ON CONFLICT(id) DO NOTHING`, public, private)
	if err != nil {
		return "", "", err
	}
	// Someone else may have raced us to the insert (e.g. two instances
	// starting up against the same fresh database at once); re-read so every
	// caller ends up with the one row that actually won.
	err = s.db.QueryRow(`SELECT public_key, private_key FROM webpush_vapid_key WHERE id = 1`).Scan(&public, &private)
	if err != nil {
		return "", "", err
	}
	return public, private, nil
}

// AddSubscription stores a new subscription (or refreshes an existing one
// for the same endpoint — a browser that re-subscribes keeps the same
// endpoint most of the time, but resets last_uid so it isn't silently
// skipped past whatever mail arrived before the resubscribe).
func (s *Store) AddSubscription(email, endpoint, p256dh, auth string, passwordEnc, passwordNonce []byte, now time.Time) error {
	_, err := s.exec(`INSERT INTO webpush_subscription (email, endpoint, p256dh, auth, password_enc, password_nonce, last_uid, created_at)
        VALUES (?, ?, ?, ?, ?, ?, 0, ?)
        ON CONFLICT(endpoint) DO UPDATE SET email = excluded.email, p256dh = excluded.p256dh, auth = excluded.auth,
            password_enc = excluded.password_enc, password_nonce = excluded.password_nonce, last_uid = 0`,
		email, endpoint, p256dh, auth, passwordEnc, passwordNonce, storage.Unix(now))
	return err
}

// RemoveSubscription deletes a subscription by its endpoint (e.g. the
// browser unsubscribed, or the push service reports it as gone).
func (s *Store) RemoveSubscription(endpoint string) error {
	_, err := s.exec(`DELETE FROM webpush_subscription WHERE endpoint = ?`, endpoint)
	return err
}

// ListByEmail returns every subscription registered for a mailbox, so
// Settings can show "3 devices subscribed" and let the user clear them.
func (s *Store) ListByEmail(email string) ([]Subscription, error) {
	rows, err := s.db.Query(s.d.Rebind(`SELECT id, email, endpoint, p256dh, auth, password_enc, password_nonce, last_uid, created_at
        FROM webpush_subscription WHERE email = ? ORDER BY id`), email)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanSubscriptions(rows)
}

// ListAll returns every subscription across every mailbox, for the
// background poller to sweep.
func (s *Store) ListAll() ([]Subscription, error) {
	rows, err := s.db.Query(`SELECT id, email, endpoint, p256dh, auth, password_enc, password_nonce, last_uid, created_at FROM webpush_subscription ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanSubscriptions(rows)
}

func scanSubscriptions(rows *sql.Rows) ([]Subscription, error) {
	var out []Subscription
	for rows.Next() {
		var sub Subscription
		var createdAt int64
		if err := rows.Scan(&sub.ID, &sub.Email, &sub.Endpoint, &sub.P256dh, &sub.Auth, &sub.PasswordEnc, &sub.PasswordNonce, &sub.LastUID, &createdAt); err != nil {
			return nil, err
		}
		sub.CreatedAt = storage.FromUnix(createdAt)
		out = append(out, sub)
	}
	return out, rows.Err()
}

// UpdateLastUID records the highest IMAP UID the poller has already
// notified about for this subscription, so the next sweep only reports mail
// newer than that.
func (s *Store) UpdateLastUID(endpoint string, uid uint32) error {
	_, err := s.exec(`UPDATE webpush_subscription SET last_uid = ? WHERE endpoint = ?`, uid, endpoint)
	return err
}
