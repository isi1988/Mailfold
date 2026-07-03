package admin

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // TOTP (RFC 6238) mandates HMAC-SHA1; this is not used for anything else.
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"time"
)

// totpDigits and totpStep are the RFC 6238 defaults every authenticator app
// (Google Authenticator, Authy, 1Password, …) assumes.
const (
	totpDigits = 6
	totpStep   = 30 * time.Second
	// totpSkew allows the previous and next 30s step to also verify, so a small
	// clock drift between the server and the user's phone does not lock them out.
	totpSkew = 1
)

// NewTOTPSecret returns a fresh 160-bit random secret, base32-encoded (no
// padding) the way authenticator apps expect it.
func NewTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}

// TOTPURI builds the otpauth:// URI an authenticator app scans (as a QR code)
// or accepts as manual entry text.
func TOTPURI(issuer, account, secret string) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	q := url.Values{
		"secret": {secret},
		"issuer": {issuer},
		"digits": {strconv.Itoa(totpDigits)},
		"period": {strconv.Itoa(int(totpStep.Seconds()))},
	}
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// VerifyTOTP reports whether code is a valid current (±1 step) TOTP code for
// secret. It never returns an error for a malformed code — a bad guess just
// fails verification like any other wrong code.
func VerifyTOTP(secret, code string) bool {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return false
	}
	now := time.Now()
	for skew := -totpSkew; skew <= totpSkew; skew++ {
		want := totpCode(key, now.Add(time.Duration(skew)*totpStep))
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

// totpCode computes the RFC 6238 code for key at time t.
func totpCode(key []byte, t time.Time) string {
	counter := uint64(t.Unix() / int64(totpStep.Seconds()))
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	trunc := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	code := trunc % uint32(math.Pow10(totpDigits))
	return fmt.Sprintf("%0*d", totpDigits, code)
}
