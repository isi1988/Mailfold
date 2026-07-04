// Package config loads Mailfold runtime configuration from the process
// environment. Centralizing configuration here means the rest of the backend
// never reads environment variables directly; it receives a fully validated
// Config value instead, which keeps configuration policy (defaults, required
// fields, parsing rules) in a single, testable place.
package config

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the complete runtime configuration for the Mailfold backend.
// Every field is populated from an environment variable by Load, so a Config
// value is an immutable snapshot of how a particular process instance should
// behave. It exists so that dependencies (the HTTP server, the mailcow client,
// the authenticator) can be constructed from one explicit source of truth
// rather than reaching into the environment on their own.
type Config struct {
	// Addr is the TCP address the HTTP server listens on, in Go's
	// "host:port" form (for example ":8080" to bind every interface on port
	// 8080). It is passed verbatim to http.Server.
	Addr string
	// MailcowBaseURL is the base URL of the upstream mailcow instance that
	// Mailfold proxies administrative requests to. It is required because
	// Mailfold has no useful behavior without a mailcow backend to talk to.
	MailcowBaseURL string
	// MailcowAPIKey is the secret API key sent to mailcow in the X-API-Key
	// header. It is kept out of the codebase and supplied only through the
	// environment so the secret never lands in source control or logs.
	MailcowAPIKey string
	// MailcowInsecureTLS, when true, disables TLS certificate verification on
	// requests to mailcow. It exists purely to support local development
	// against self-signed certificates and must never be enabled in
	// production, where it would allow man-in-the-middle attacks.
	MailcowInsecureTLS bool
	// FrontendDir is the filesystem path to the built single-page frontend
	// application. It is optional; when the directory is absent the backend
	// still serves its API, it simply has no static assets to hand out.
	FrontendDir string
	// AdminUser is the username of the single Mailfold administrator account.
	// Mailfold performs its own lightweight authentication in front of mailcow
	// rather than exposing the mailcow API key to browsers.
	AdminUser string
	// AdminPassword is the plaintext password for the administrator account as
	// read from the environment. It is required so the deployment operator is
	// forced to choose a credential rather than accidentally running with an
	// empty or default password.
	AdminPassword string
	// SessionTTL is how long an authenticated session (bearer token) remains
	// valid after it is issued. It bounds the blast radius of a leaked token
	// by guaranteeing tokens expire even if a client never logs out.
	SessionTTL time.Duration
	// CORSOrigins is the list of browser origins allowed to call the API. The
	// special value "*" permits any origin; listing explicit origins locks the
	// API down to known frontends.
	CORSOrigins []string
	// LoginRateMax is the maximum number of login attempts permitted per client
	// IP within LoginRateWindow before further attempts are rejected with HTTP
	// 429. It throttles password brute-force attacks. A value <= 0 disables the
	// limit.
	LoginRateMax int
	// LoginRateWindow is the fixed window over which LoginRateMax is counted.
	LoginRateWindow time.Duration
	// MaxBodyBytes caps the size of a request body the server will read, guarding
	// against memory exhaustion from oversized or malicious payloads. A value
	// <= 0 leaves the body size unbounded.
	MaxBodyBytes int64
	// IMAPAddr is the "host:port" of the IMAP server the webmail layer connects
	// to. When empty, webmail is disabled and its endpoints report 503.
	IMAPAddr string
	// SMTPAddr is the "host:port" of the SMTP submission server used to send mail.
	SMTPAddr string
	// MailUseTLS selects implicit TLS (IMAPS/SMTPS) for webmail connections; when
	// false a plaintext connection is used.
	MailUseTLS bool
	// MailInsecureTLS skips TLS certificate verification for webmail connections
	// (development only, e.g. against self-signed mailcow certificates).
	MailInsecureTLS bool
	// WebmailSessionTTL is the lifetime of an authenticated webmail session.
	WebmailSessionTTL time.Duration
	// DBDriver selects the database backend for the DAV and API-key stores. The
	// open-source build supports only "sqlite" (the default); the enterprise build
	// additionally supports "postgres". Requesting an unavailable driver fails
	// fast with a clear error at startup.
	DBDriver string
	// DBPath is the DSN for the database backing the CardDAV/CalDAV and API-key
	// stores — a file path for SQLite, or a connection string for other drivers.
	// When empty, the stores (and thus DAV/API keys) are disabled.
	DBPath string
	// APIKeyEnabled turns on the machine-to-machine API-key subsystem (durable
	// bearer keys for sending and collecting mail over the REST API). It reuses
	// DBPath for storage and self-disables when that is empty.
	APIKeyEnabled bool
	// APIKeyMasterKey is the decoded master key (>= 32 bytes) from which the
	// AES-256-GCM key that encrypts stored mailcow app-passwords is derived. It is
	// required, and validated, only when APIKeyEnabled is true, and is never
	// logged.
	APIKeyMasterKey []byte
	// APIKeyRateMax and APIKeyRateWindow bound how many API requests a single key
	// may make per window before receiving HTTP 429.
	APIKeyRateMax    int
	APIKeyRateWindow time.Duration
	// APIKeyDefaultTTL is applied as the key expiry at mint time when the request
	// omits one (0 = never expires).
	APIKeyDefaultTTL time.Duration
	// APIKeyMaxRecipients caps the total To+Cc+Bcc count a single send may target,
	// bounding an authorized key's use as a bulk mailer.
	APIKeyMaxRecipients int
	// ServerName is the public mail-server hostname shown in the UI's status
	// indicator (for example "mail.example.com"). It is display-only and defaults
	// to empty, in which case the UI simply omits the hostname.
	ServerName string

	// AdminEncKey is the decoded master key (>= 32 bytes) from which the
	// AES-256-GCM key that encrypts every secret Mailfold stores at rest is
	// derived: the admin's TOTP seed and notification-sender mailbox password,
	// a webmail user's own TOTP seed, an SSO provider's OIDC client secret,
	// and a mailbox's cached SSO app-password. It is optional: when unset,
	// every one of those features reports 501 rather than failing, but
	// everything else — password change, profile, sessions — still works off
	// DBPath alone.
	AdminEncKey []byte
}

// Load reads every configuration value from the environment, applies sensible
// defaults for optional settings, and validates that all required settings are
// present. It returns a fully populated Config on success, or an error naming
// the first missing required variable so that misconfiguration fails fast at
// startup rather than surfacing as a confusing runtime failure later.
func Load() (*Config, error) {
	cfg := &Config{
		Addr:                getenv("MAILFOLD_ADDR", ":8080"),
		MailcowBaseURL:      getenv("MAILFOLD_MAILCOW_URL", ""),
		MailcowAPIKey:       os.Getenv("MAILFOLD_MAILCOW_API_KEY"),
		MailcowInsecureTLS:  getbool("MAILFOLD_MAILCOW_INSECURE_TLS", false),
		FrontendDir:         getenv("MAILFOLD_FRONTEND_DIR", "./frontend/dist"),
		AdminUser:           getenv("MAILFOLD_ADMIN_USER", "admin"),
		AdminPassword:       os.Getenv("MAILFOLD_ADMIN_PASSWORD"),
		SessionTTL:          getdur("MAILFOLD_SESSION_TTL", 12*time.Hour),
		CORSOrigins:         getlist("MAILFOLD_CORS_ORIGINS", []string{"*"}),
		LoginRateMax:        int(getint64("MAILFOLD_LOGIN_RATE_MAX", 5)),
		LoginRateWindow:     getdur("MAILFOLD_LOGIN_RATE_WINDOW", time.Minute),
		MaxBodyBytes:        getint64("MAILFOLD_MAX_BODY_BYTES", 25<<20),
		IMAPAddr:            os.Getenv("MAILFOLD_IMAP_ADDR"),
		SMTPAddr:            os.Getenv("MAILFOLD_SMTP_ADDR"),
		MailUseTLS:          getbool("MAILFOLD_MAIL_TLS", true),
		MailInsecureTLS:     getbool("MAILFOLD_MAIL_INSECURE_TLS", false),
		WebmailSessionTTL:   getdur("MAILFOLD_WEBMAIL_SESSION_TTL", 12*time.Hour),
		DBDriver:            getenv("MAILFOLD_DB_DRIVER", "sqlite"),
		DBPath:              os.Getenv("MAILFOLD_DB_PATH"),
		APIKeyEnabled:       getbool("MAILFOLD_APIKEY_ENABLED", false),
		APIKeyRateMax:       int(getint64("MAILFOLD_APIKEY_RATE_MAX", 120)),
		APIKeyRateWindow:    getdur("MAILFOLD_APIKEY_RATE_WINDOW", time.Minute),
		APIKeyDefaultTTL:    getdur("MAILFOLD_APIKEY_DEFAULT_TTL", 0),
		APIKeyMaxRecipients: int(getint64("MAILFOLD_APIKEY_MAX_RECIPIENTS", 50)),
		ServerName:          getenv("MAILFOLD_SERVER_NAME", ""),
	}

	// The following three values have no safe default: without an upstream
	// mailcow target, its API key, and an admin password there is nothing the
	// backend can usefully or securely do, so a missing value is a hard error.
	if cfg.MailcowBaseURL == "" {
		return nil, fmt.Errorf("MAILFOLD_MAILCOW_URL is required")
	}
	if cfg.MailcowAPIKey == "" {
		return nil, fmt.Errorf("MAILFOLD_MAILCOW_API_KEY is required")
	}
	if cfg.AdminPassword == "" {
		return nil, fmt.Errorf("MAILFOLD_ADMIN_PASSWORD is required")
	}

	// The API-key subsystem needs an encryption master key. It is only required
	// when the feature is enabled, so operators who never turn it on are not
	// forced to configure it.
	if cfg.APIKeyEnabled {
		key, err := decodeMasterKey(os.Getenv("MAILFOLD_APIKEY_MASTER_KEY"))
		if err != nil {
			return nil, err
		}
		cfg.APIKeyMasterKey = key
	}

	// The admin-account encryption key is optional. When the operator sets it,
	// it must be valid (fail fast); when unset, the admin-secret features it
	// gates simply stay off.
	if raw := os.Getenv("MAILFOLD_ADMIN_ENC_KEY"); raw != "" {
		key, err := decodeOptionalKey(raw)
		if err != nil {
			return nil, fmt.Errorf("MAILFOLD_ADMIN_ENC_KEY must decode (hex or base64) to at least 32 bytes")
		}
		cfg.AdminEncKey = key
	}
	return cfg, nil
}

// decodeOptionalKey decodes a hex- or base64-encoded key of at least 32 bytes.
// Unlike decodeMasterKey it never treats an empty string specially — callers
// only invoke it once they know the raw value is non-empty.
func decodeOptionalKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	for _, dec := range []func(string) ([]byte, error){
		hex.DecodeString,
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
	} {
		if b, err := dec(raw); err == nil && len(b) >= 32 {
			return b, nil
		}
	}
	return nil, fmt.Errorf("key too short or not hex/base64")
}

// decodeMasterKey parses the API-key master key from its environment string,
// accepting either hex or base64 (standard or raw) and requiring at least 32
// decoded bytes. It returns a clear error rather than a weak key so a
// misconfigured deployment fails fast at startup.
func decodeMasterKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("MAILFOLD_APIKEY_MASTER_KEY is required (>=32 bytes, hex or base64) when MAILFOLD_APIKEY_ENABLED=true")
	}
	for _, dec := range []func(string) ([]byte, error){
		hex.DecodeString,
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
	} {
		if b, err := dec(raw); err == nil && len(b) >= 32 {
			return b, nil
		}
	}
	return nil, fmt.Errorf("MAILFOLD_APIKEY_MASTER_KEY must decode (hex or base64) to at least 32 bytes")
}

// getenv returns the value of the environment variable named key, or def when
// that variable is unset or empty. It exists so callers can express an optional
// string setting with a fallback in a single expression. Note that an empty
// value is treated identically to an unset one, which is what we want for
// string settings where "" carries no meaning.
func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getbool reads a boolean setting from the environment, returning def when the
// variable is unset or cannot be parsed. It centralizes boolean parsing so that
// every flag accepts the same range of truthy spellings (1/t/T/true, and so on)
// that strconv.ParseBool understands. Parse failures fall back to the default
// rather than aborting, so a typo in an optional flag never crashes startup.
func getbool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// getdur reads a time.Duration setting from the environment, returning def when
// the variable is unset or is not a valid Go duration string (for example
// "12h" or "30m"). Like getbool, it fails soft to the default on a parse error
// so that a malformed optional value degrades gracefully instead of preventing
// the process from starting.
func getdur(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

// getint64 reads an integer setting from the environment, returning def when
// the variable is unset or is not a valid base-10 integer. Like the other
// parsers it fails soft to the default on error so a malformed optional value
// never prevents startup.
func getint64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

// getlist reads a comma-separated list setting from the environment and returns
// it as a slice of trimmed, non-empty entries. It returns def when the variable
// is unset, or when the variable is set but contains only whitespace and empty
// entries. This lets a configuration such as "a, b ,c" become the clean slice
// {"a", "b", "c"} while an accidentally blank value still yields the intended
// default rather than an empty list.
func getlist(key string, def []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	parts := strings.Split(v, ",")
	// Pre-size the result to the number of comma-separated fields; some may be
	// dropped below, but this avoids repeated reallocation in the common case.
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		// Trim surrounding whitespace and skip empty fields so that stray
		// commas or padding spaces do not produce blank list entries.
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	// If every field was blank the variable carried no usable value, so fall
	// back to the default rather than returning an empty (and useless) list.
	if len(out) == 0 {
		return def
	}
	return out
}
