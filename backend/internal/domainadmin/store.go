// Package domainadmin persists everything Mailfold's own login for domain
// admins needs, on top of mailcow's native domain-admin accounts (which
// otherwise only ever log into mailcow's own SOGo/admin UI, never Mailfold
// itself):
//
//   - a bcrypt password hash per domain-admin username, captured whenever a
//     super-admin sets/changes that domain admin's password through
//     Mailfold's own "New domain admin" form (see internal/api/domainadmins.go)
//   - SSO providers (OIDC), each either shared across every domain or scoped
//     to a specific set of domains, so a super-admin can offer providers that
//     domain admins pick up automatically, while domain admins can also add
//     their own scoped to just the domains they manage
//   - a cached mailcow app-password per mailbox, minted the first time
//     someone signs into that mailbox's webmail via SSO (see
//     internal/api/sso.go), so subsequent SSO logins reuse it instead of
//     minting a fresh one every time
//
// It runs on the same database as the admin/DAV/API-key/webmail-user stores,
// following their exact Open/migrate/Dialect pattern.
package domainadmin

import (
	"database/sql"
	"time"

	"github.com/isi1988/Mailfold/backend/storage"
)

// Provider is one configured OIDC identity provider.
type Provider struct {
	ID                int64
	Name              string
	Issuer            string
	ClientID          string
	ClientSecretEnc   []byte
	ClientSecretNonce []byte
	RedirectURL       string
	// AllDomains, when true, makes the provider available to every domain
	// regardless of Domains. When false, it is only available to the domains
	// listed in Domains.
	AllDomains bool
	// CreatedBy is either "" (created by the super-admin) or a domain-admin
	// username (created by that domain admin for their own domain(s), and
	// only editable/deletable by them).
	CreatedBy string
	Active    bool
	Domains   []string // resolved separately from sso_provider_domain
	UpdatedAt time.Time
}

// MailboxCredential is a cached mailcow app-password used to operate a
// mailbox's IMAP/SMTP session after an SSO login, which never has (or needs)
// the mailbox's real password.
type MailboxCredential struct {
	Mailbox        string
	AppPasswdID    string
	AppPasswdEnc   []byte
	AppPasswdNonce []byte
	UpdatedAt      time.Time
}

// Store is the persistence layer for domain-admin login and SSO providers.
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
		`CREATE TABLE IF NOT EXISTS domain_admin_login (
    username       TEXT NOT NULL PRIMARY KEY,
    password_hash  TEXT NOT NULL DEFAULT '',
    updated_at     ` + s.d.IntType() + ` NOT NULL DEFAULT 0
)`,
		`CREATE TABLE IF NOT EXISTS sso_provider (
    id                   ` + s.d.IntType() + ` PRIMARY KEY,
    name                 TEXT NOT NULL DEFAULT '',
    issuer               TEXT NOT NULL DEFAULT '',
    client_id            TEXT NOT NULL DEFAULT '',
    client_secret_enc    ` + s.d.BlobType() + `,
    client_secret_nonce  ` + s.d.BlobType() + `,
    redirect_url         TEXT NOT NULL DEFAULT '',
    all_domains          INTEGER NOT NULL DEFAULT 0,
    created_by           TEXT NOT NULL DEFAULT '',
    active               INTEGER NOT NULL DEFAULT 1,
    updated_at           ` + s.d.IntType() + ` NOT NULL DEFAULT 0
)`,
		`CREATE TABLE IF NOT EXISTS sso_provider_domain (
    provider_id  ` + s.d.IntType() + ` NOT NULL,
    domain_name  TEXT NOT NULL,
    PRIMARY KEY (provider_id, domain_name)
)`,
		`CREATE TABLE IF NOT EXISTS sso_mailbox_credential (
    mailbox             TEXT NOT NULL PRIMARY KEY,
    app_passwd_id       TEXT NOT NULL DEFAULT '',
    app_passwd_enc      ` + s.d.BlobType() + `,
    app_passwd_nonce    ` + s.d.BlobType() + `,
    updated_at          ` + s.d.IntType() + ` NOT NULL DEFAULT 0
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

// --- Domain-admin login ---

// SetLoginPassword stores (or replaces) a domain admin's password hash.
func (s *Store) SetLoginPassword(username, hash string, now time.Time) error {
	_, err := s.exec(`INSERT INTO domain_admin_login (username, password_hash, updated_at) VALUES (?, ?, ?)
        ON CONFLICT(username) DO UPDATE SET password_hash = excluded.password_hash, updated_at = excluded.updated_at`,
		username, hash, storage.Unix(now))
	return err
}

// GetLoginPassword returns username's stored password hash, if any.
func (s *Store) GetLoginPassword(username string) (string, bool, error) {
	var hash string
	err := s.db.QueryRow(s.d.Rebind(`SELECT password_hash FROM domain_admin_login WHERE username = ?`), username).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return hash, true, nil
}

// DeleteLogin removes a domain admin's stored login, e.g. when the domain
// admin itself is deleted.
func (s *Store) DeleteLogin(username string) error {
	_, err := s.exec(`DELETE FROM domain_admin_login WHERE username = ?`, username)
	return err
}

// --- SSO providers ---

// CreateProvider inserts a new provider (with its domain scope, when not
// AllDomains) and returns its id.
func (s *Store) CreateProvider(p Provider, now time.Time) (int64, error) {
	res, err := s.exec(`INSERT INTO sso_provider
        (name, issuer, client_id, client_secret_enc, client_secret_nonce, redirect_url, all_domains, created_by, active, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.Issuer, p.ClientID, p.ClientSecretEnc, p.ClientSecretNonce, p.RedirectURL,
		boolInt(p.AllDomains), p.CreatedBy, boolInt(p.Active), storage.Unix(now))
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if !p.AllDomains {
		if err := s.replaceProviderDomains(id, p.Domains); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (s *Store) replaceProviderDomains(id int64, domains []string) error {
	if _, err := s.exec(`DELETE FROM sso_provider_domain WHERE provider_id = ?`, id); err != nil {
		return err
	}
	for _, d := range domains {
		if _, err := s.exec(`INSERT INTO sso_provider_domain (provider_id, domain_name) VALUES (?, ?)`, id, d); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) domainsFor(id int64) ([]string, error) {
	rows, err := s.db.Query(s.d.Rebind(`SELECT domain_name FROM sso_provider_domain WHERE provider_id = ? ORDER BY domain_name`), id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// scanProviderRow scans only sso_provider's own columns — it must never run a
// nested query (e.g. to resolve Domains), because a caller iterating rows via
// rows.Next() still holds the connection open, and the sqlite driver is
// configured with exactly one connection in the pool (see storage/sqlite.go):
// a nested query on the same *sql.DB would block forever waiting for a
// connection that only frees up once the outer rows are closed. Domains are
// resolved in a second pass, after the first query's rows are fully done.
func scanProviderRow(row scanner) (Provider, error) {
	var p Provider
	var allDomains, active int
	var updatedAt int64
	err := row.Scan(&p.ID, &p.Name, &p.Issuer, &p.ClientID, &p.ClientSecretEnc, &p.ClientSecretNonce,
		&p.RedirectURL, &allDomains, &p.CreatedBy, &active, &updatedAt)
	if err != nil {
		return Provider{}, err
	}
	p.AllDomains = allDomains != 0
	p.Active = active != 0
	p.UpdatedAt = storage.FromUnix(updatedAt)
	return p, nil
}

// fillDomains resolves and attaches Domains to every non-AllDomains provider
// in list. Called only after the query that produced list has been fully
// consumed and closed.
func (s *Store) fillDomains(list []Provider) error {
	for i := range list {
		if list[i].AllDomains {
			continue
		}
		domains, err := s.domainsFor(list[i].ID)
		if err != nil {
			return err
		}
		list[i].Domains = domains
	}
	return nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

const providerColumns = `id, name, issuer, client_id, client_secret_enc, client_secret_nonce, redirect_url, all_domains, created_by, active, updated_at`

// ListProviders returns every provider, most recently updated first.
func (s *Store) ListProviders() ([]Provider, error) {
	rows, err := s.db.Query(`SELECT ` + providerColumns + ` FROM sso_provider ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	var out []Provider
	for rows.Next() {
		p, err := scanProviderRow(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := s.fillDomains(out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetProvider returns a single provider by id. QueryRow's Scan fully
// consumes and releases its connection before this returns, so the domainsFor
// lookup afterward is safe even with a single-connection pool.
func (s *Store) GetProvider(id int64) (Provider, bool, error) {
	row := s.db.QueryRow(s.d.Rebind(`SELECT `+providerColumns+` FROM sso_provider WHERE id = ?`), id)
	p, err := scanProviderRow(row)
	if err == sql.ErrNoRows {
		return Provider{}, false, nil
	}
	if err != nil {
		return Provider{}, false, err
	}
	if !p.AllDomains {
		domains, err := s.domainsFor(p.ID)
		if err != nil {
			return Provider{}, false, err
		}
		p.Domains = domains
	}
	return p, true, nil
}

// ProvidersForDomain returns every active provider available to domain:
// every AllDomains provider, plus every provider explicitly scoped to it.
func (s *Store) ProvidersForDomain(domain string) ([]Provider, error) {
	rows, err := s.db.Query(s.d.Rebind(`
        SELECT `+providerColumns+` FROM sso_provider
        WHERE active = 1 AND (
            all_domains = 1
            OR id IN (SELECT provider_id FROM sso_provider_domain WHERE domain_name = ?)
        )
        ORDER BY name`), domain)
	if err != nil {
		return nil, err
	}
	var out []Provider
	for rows.Next() {
		p, err := scanProviderRow(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := s.fillDomains(out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateProvider replaces a provider's fields (including its domain scope).
// Pass the existing ClientSecretEnc/Nonce to keep the current secret
// unchanged (the API layer does this when the caller left the secret blank).
func (s *Store) UpdateProvider(p Provider, now time.Time) error {
	_, err := s.exec(`UPDATE sso_provider SET
        name = ?, issuer = ?, client_id = ?, client_secret_enc = ?, client_secret_nonce = ?,
        redirect_url = ?, all_domains = ?, active = ?, updated_at = ?
        WHERE id = ?`,
		p.Name, p.Issuer, p.ClientID, p.ClientSecretEnc, p.ClientSecretNonce,
		p.RedirectURL, boolInt(p.AllDomains), boolInt(p.Active), storage.Unix(now), p.ID)
	if err != nil {
		return err
	}
	if !p.AllDomains {
		return s.replaceProviderDomains(p.ID, p.Domains)
	}
	return s.replaceProviderDomains(p.ID, nil)
}

// DeleteProvider removes a provider and its domain scope rows.
func (s *Store) DeleteProvider(id int64) error {
	if _, err := s.exec(`DELETE FROM sso_provider_domain WHERE provider_id = ?`, id); err != nil {
		return err
	}
	_, err := s.exec(`DELETE FROM sso_provider WHERE id = ?`, id)
	return err
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// --- Mailbox SSO credential cache ---

// GetMailboxCredential returns the cached app-password credential for
// mailbox, if one has been minted before.
func (s *Store) GetMailboxCredential(mailbox string) (MailboxCredential, bool, error) {
	var c MailboxCredential
	c.Mailbox = mailbox
	var updatedAt int64
	err := s.db.QueryRow(s.d.Rebind(`SELECT app_passwd_id, app_passwd_enc, app_passwd_nonce, updated_at
        FROM sso_mailbox_credential WHERE mailbox = ?`), mailbox).
		Scan(&c.AppPasswdID, &c.AppPasswdEnc, &c.AppPasswdNonce, &updatedAt)
	if err == sql.ErrNoRows {
		return MailboxCredential{}, false, nil
	}
	if err != nil {
		return MailboxCredential{}, false, err
	}
	c.UpdatedAt = storage.FromUnix(updatedAt)
	return c, true, nil
}

// SetMailboxCredential stores (or replaces) the cached app-password
// credential for mailbox.
func (s *Store) SetMailboxCredential(mailbox, appPwID string, enc, nonce []byte, now time.Time) error {
	_, err := s.exec(`INSERT INTO sso_mailbox_credential (mailbox, app_passwd_id, app_passwd_enc, app_passwd_nonce, updated_at)
        VALUES (?, ?, ?, ?, ?)
        ON CONFLICT(mailbox) DO UPDATE SET app_passwd_id = excluded.app_passwd_id,
            app_passwd_enc = excluded.app_passwd_enc, app_passwd_nonce = excluded.app_passwd_nonce, updated_at = excluded.updated_at`,
		mailbox, appPwID, enc, nonce, storage.Unix(now))
	return err
}

// DeleteMailboxCredential removes a mailbox's cached SSO credential.
func (s *Store) DeleteMailboxCredential(mailbox string) error {
	_, err := s.exec(`DELETE FROM sso_mailbox_credential WHERE mailbox = ?`, mailbox)
	return err
}
