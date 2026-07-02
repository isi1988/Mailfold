// Package config loads Mailfold runtime configuration from the process
// environment. Centralizing configuration here means the rest of the backend
// never reads environment variables directly; it receives a fully validated
// Config value instead, which keeps configuration policy (defaults, required
// fields, parsing rules) in a single, testable place.
package config

import (
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
}

// Load reads every configuration value from the environment, applies sensible
// defaults for optional settings, and validates that all required settings are
// present. It returns a fully populated Config on success, or an error naming
// the first missing required variable so that misconfiguration fails fast at
// startup rather than surfacing as a confusing runtime failure later.
func Load() (*Config, error) {
	cfg := &Config{
		Addr:               getenv("MAILFOLD_ADDR", ":8080"),
		MailcowBaseURL:     getenv("MAILFOLD_MAILCOW_URL", ""),
		MailcowAPIKey:      os.Getenv("MAILFOLD_MAILCOW_API_KEY"),
		MailcowInsecureTLS: getbool("MAILFOLD_MAILCOW_INSECURE_TLS", false),
		FrontendDir:        getenv("MAILFOLD_FRONTEND_DIR", "./frontend/dist"),
		AdminUser:          getenv("MAILFOLD_ADMIN_USER", "admin"),
		AdminPassword:      os.Getenv("MAILFOLD_ADMIN_PASSWORD"),
		SessionTTL:         getdur("MAILFOLD_SESSION_TTL", 12*time.Hour),
		CORSOrigins:        getlist("MAILFOLD_CORS_ORIGINS", []string{"*"}),
		LoginRateMax:       int(getint64("MAILFOLD_LOGIN_RATE_MAX", 5)),
		LoginRateWindow:    getdur("MAILFOLD_LOGIN_RATE_WINDOW", time.Minute),
		MaxBodyBytes:       getint64("MAILFOLD_MAX_BODY_BYTES", 1<<20),
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
	return cfg, nil
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
