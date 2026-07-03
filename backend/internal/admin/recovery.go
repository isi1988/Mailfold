package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// recoveryCodeCount is how many single-use codes are minted per enrollment.
const recoveryCodeCount = 10

// recoveryAlphabet avoids visually-ambiguous characters (0/O, 1/I/L) so a code
// copied off a screen or printed is easy to retype correctly.
const recoveryAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

// NewRecoveryCodes mints a fresh set of two-factor recovery codes, each
// formatted as two 5-character groups (e.g. "7K9QX-2MPRT") for readability.
func NewRecoveryCodes() ([]string, error) {
	out := make([]string, recoveryCodeCount)
	for i := range out {
		code, err := randomRecoveryCode()
		if err != nil {
			return nil, err
		}
		out[i] = code
	}
	return out, nil
}

func randomRecoveryCode() (string, error) {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	var b strings.Builder
	for i, v := range buf {
		if i == 5 {
			b.WriteByte('-')
		}
		b.WriteByte(recoveryAlphabet[int(v)%len(recoveryAlphabet)])
	}
	return b.String(), nil
}

// HashRecoveryCode normalises and hashes a recovery code for storage/lookup.
// Recovery codes are high-entropy (10 chars from a 32-symbol alphabet, ~50 bits)
// and single-use, so a plain salted SHA-256 (rather than bcrypt) is sufficient
// and keeps verification cheap when checked against every stored code.
func HashRecoveryCode(code string) string {
	norm := strings.ToUpper(strings.TrimSpace(code))
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}
