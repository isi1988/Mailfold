package admin

import (
	"bytes"
	"encoding/base32"
	"testing"
	"time"
)

func TestCipherRoundTrip(t *testing.T) {
	if _, err := NewCipher([]byte("too short")); err == nil {
		t.Error("NewCipher must reject a short master key")
	}
	master := bytes.Repeat([]byte{0x11}, 32)
	c, err := NewCipher(master)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	ciphertext, nonce, err := c.Seal([]byte("s3cr3t"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	plain, err := c.Open(ciphertext, nonce)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if string(plain) != "s3cr3t" {
		t.Errorf("Open = %q", plain)
	}
	if _, err := c.Open(ciphertext, []byte("wrong-size-nonce")); err == nil {
		t.Error("Open should reject a wrong-size nonce")
	}
	other := bytes.Repeat([]byte{0x22}, 32)
	c2, _ := NewCipher(other)
	if _, err := c2.Open(ciphertext, nonce); err == nil {
		t.Error("Open should fail when decrypted with a different key")
	}
}

func TestTOTPGenerateAndVerify(t *testing.T) {
	secret, err := NewTOTPSecret()
	if err != nil {
		t.Fatalf("NewTOTPSecret: %v", err)
	}
	if secret == "" {
		t.Fatal("expected a non-empty secret")
	}
	code := totpCode(mustDecodeSecret(t, secret), time.Now())
	if !VerifyTOTP(secret, code) {
		t.Error("current code should verify")
	}
	if VerifyTOTP(secret, "000000") && code != "000000" {
		t.Error("an arbitrary wrong code should not verify")
	}
	if VerifyTOTP("not-valid-base32!!", code) {
		t.Error("a malformed secret should never verify")
	}

	// Skew tolerance: the previous step's code should still verify.
	prevCode := totpCode(mustDecodeSecret(t, secret), time.Now().Add(-totpStep))
	if !VerifyTOTP(secret, prevCode) {
		t.Error("a code from one step ago should still verify (skew tolerance)")
	}
	// But a code far outside the skew window should not.
	farCode := totpCode(mustDecodeSecret(t, secret), time.Now().Add(-10*totpStep))
	if VerifyTOTP(secret, farCode) && farCode != code && farCode != prevCode {
		t.Error("a code far outside the skew window should not verify")
	}
}

func TestTOTPURI(t *testing.T) {
	uri := TOTPURI("Mailfold", "admin", "ABCDEF")
	if uri == "" {
		t.Fatal("expected a non-empty URI")
	}
	if uri[:14] != "otpauth://totp" {
		t.Errorf("unexpected URI prefix: %q", uri)
	}
}

func mustDecodeSecret(t *testing.T, secret string) []byte {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	return key
}

func TestRecoveryCodes(t *testing.T) {
	codes, err := NewRecoveryCodes()
	if err != nil {
		t.Fatalf("NewRecoveryCodes: %v", err)
	}
	if len(codes) != recoveryCodeCount {
		t.Fatalf("want %d codes, got %d", recoveryCodeCount, len(codes))
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if len(c) != 11 || c[5] != '-' {
			t.Errorf("unexpected code format: %q", c)
		}
		if seen[c] {
			t.Errorf("duplicate code minted: %q", c)
		}
		seen[c] = true
	}

	h1 := HashRecoveryCode(codes[0])
	h2 := HashRecoveryCode(" " + codes[0] + " ")
	h3 := HashRecoveryCode(codesToLower(codes[0]))
	if h1 != h2 || h1 != h3 {
		t.Error("HashRecoveryCode should normalise whitespace and case")
	}
	if HashRecoveryCode(codes[0]) == HashRecoveryCode(codes[1]) {
		t.Error("distinct codes should hash distinctly")
	}
}

func codesToLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
