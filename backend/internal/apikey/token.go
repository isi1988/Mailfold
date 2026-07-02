// Package apikey implements Mailfold's machine-to-machine API keys: durable,
// individually-revocable bearer tokens that let third-party services send and
// collect mail for a single mailbox through the REST API. Each key is a thin
// handle in front of a mailcow application password (scoped to IMAP+SMTP only);
// the real mailbox password is never stored or seen. This package is pure
// persistence and crypto — it has no mailcow or HTTP dependencies — so the
// orchestration (minting the upstream app-password, wiring HTTP routes) lives in
// the api package on top of it.
package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

// TokenPrefix marks every Mailfold API token. It is a stable, greppable string
// so secret scanners (gitleaks, GitHub push protection) can flag a leak.
const TokenPrefix = "mf_live_"

// PrefixDisplayLen is how many leading characters of a token are kept as a
// non-secret label for the management UI ("mf_live_<8 hex of kid>").
const PrefixDisplayLen = 16

// ErrMalformedToken is returned when a presented token is not a well-formed
// Mailfold token (wrong prefix or wrong segment count).
var ErrMalformedToken = errors.New("malformed api token")

// NewToken mints a fresh token. It returns the full token string (shown to the
// caller exactly once), its public key id (kid, also the store primary key), the
// hex SHA-256 of the token (the only representation persisted), and a short
// display prefix. The secret half has 256 bits of entropy, so a plain unsalted
// SHA-256 is a sound at-rest representation — brute-forcing it is infeasible and
// bcrypt would only add latency to every API request.
func NewToken() (token, kid, sha, prefix string, err error) {
	kidBytes := make([]byte, 8)
	if _, err = rand.Read(kidBytes); err != nil {
		return "", "", "", "", err
	}
	secretBytes := make([]byte, 32)
	if _, err = rand.Read(secretBytes); err != nil {
		return "", "", "", "", err
	}
	kid = hex.EncodeToString(kidBytes)
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)
	token = TokenPrefix + kid + "_" + secret
	sha = HashToken(token)
	prefix = token[:PrefixDisplayLen]
	return token, kid, sha, prefix, nil
}

// HashToken returns the hex SHA-256 of a token, the form stored and compared.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ParseKID validates a presented token's shape and extracts its key id. The
// token must carry the Mailfold prefix and split into exactly a kid and a secret
// after it, so a lookup is a single primary-key hit. It deliberately does not
// touch any stored data, so an attacker learns nothing from a shape rejection.
func ParseKID(token string) (string, error) {
	if !strings.HasPrefix(token, TokenPrefix) {
		return "", ErrMalformedToken
	}
	rest := token[len(TokenPrefix):]
	// The secret half is base64url and may itself contain '_', so split only on
	// the first separator: kid before it, secret after.
	parts := strings.SplitN(rest, "_", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", ErrMalformedToken
	}
	return parts[0], nil
}

// TokenMatches reports whether a presented token hashes to the stored hash,
// using a constant-time comparison so verification leaks no timing signal.
func TokenMatches(presented, storedSHA string) bool {
	got := HashToken(presented)
	return subtle.ConstantTimeCompare([]byte(got), []byte(storedSHA)) == 1
}
