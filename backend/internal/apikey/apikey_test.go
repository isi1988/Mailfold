package apikey

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestTokenLifecycle(t *testing.T) {
	token, kid, sha, prefix, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if !strings.HasPrefix(token, TokenPrefix) {
		t.Errorf("token %q missing prefix", token)
	}
	if len(prefix) != PrefixDisplayLen || !strings.HasPrefix(token, prefix) {
		t.Errorf("bad display prefix %q", prefix)
	}
	if got, err := ParseKID(token); err != nil || got != kid {
		t.Errorf("ParseKID = %q, %v; want %q", got, err, kid)
	}
	if HashToken(token) != sha {
		t.Error("HashToken mismatch")
	}
	if !TokenMatches(token, sha) {
		t.Error("TokenMatches should accept the real token")
	}
	if TokenMatches(token+"x", sha) {
		t.Error("TokenMatches must reject a wrong token")
	}

	// Two mints must differ.
	token2, kid2, _, _, _ := NewToken()
	if token == token2 || kid == kid2 {
		t.Error("expected distinct tokens/kids")
	}
}

func TestParseKIDErrors(t *testing.T) {
	for _, tok := range []string{"", "nope", "mf_live_", "mf_live_onlykid", "mf_live__"} {
		if _, err := ParseKID(tok); err == nil {
			t.Errorf("ParseKID(%q) should error", tok)
		}
	}
}

func TestScopes(t *testing.T) {
	if len(DefaultScopes()) != 3 {
		t.Error("default scopes should grant send/read/write")
	}
	if !ValidScope(ScopeSend) || ValidScope("mail:admin") {
		t.Error("ValidScope wrong")
	}
	joined := JoinScopes([]string{ScopeRead, ScopeWrite})
	if joined != "mail:read,mail:write" {
		t.Errorf("JoinScopes = %q", joined)
	}
	got := SplitScopes(" mail:read , mail:write ,")
	if len(got) != 2 || got[0] != ScopeRead || got[1] != ScopeWrite {
		t.Errorf("SplitScopes = %v", got)
	}
	if SplitScopes("") != nil {
		t.Error("SplitScopes(\"\") should be nil")
	}
	if !HasScope(joined, ScopeRead) || HasScope(joined, ScopeSend) {
		t.Error("HasScope wrong")
	}
}

func TestCipher(t *testing.T) {
	if _, err := NewCipher([]byte("too short")); err == nil {
		t.Error("NewCipher must reject a short master key")
	}
	master := bytes.Repeat([]byte{0xab}, 32)
	c, err := NewCipher(master)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	plain := []byte("app-password-secret")
	enc, nonce, err := c.Seal(plain)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Contains(enc, plain) {
		t.Error("ciphertext must not contain plaintext")
	}
	got, err := c.Open(enc, nonce)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("Open roundtrip failed: %v", err)
	}
	// Tampered ciphertext must fail authentication.
	enc[0] ^= 0xff
	if _, err := c.Open(enc, nonce); err == nil {
		t.Error("Open must reject tampered ciphertext")
	}
	if _, err := c.Open([]byte("x"), []byte("short")); err == nil {
		t.Error("Open must reject a wrong-size nonce")
	}
	// Two seals of the same plaintext use distinct nonces.
	_, n1, _ := c.Seal(plain)
	_, n2, _ := c.Seal(plain)
	if bytes.Equal(n1, n2) {
		t.Error("nonces should be unique per seal")
	}
	// A cipher from a different master key cannot decrypt.
	other, _ := NewCipher(bytes.Repeat([]byte{0x01}, 32))
	if _, err := other.Open(enc, nonce); err == nil {
		t.Error("a different master key must not decrypt")
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open("sqlite", t.TempDir()+"/keys.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestStoreCRUD(t *testing.T) {
	st := openTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	rec := Record{
		ID:          "kid1",
		TokenSHA256: "deadbeef",
		Prefix:      "mf_live_kid1",
		Mailbox:     "u@example.com",
		Label:       "bot",
		Scopes:      JoinScopes(DefaultScopes()),
		SecretEnc:   []byte{1, 2, 3},
		SecretNonce: []byte{4, 5, 6},
		MCAppPwID:   "7",
		Created:     now,
	}
	if err := st.Create(rec); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := st.GetByID("kid1")
	if err != nil || got == nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Mailbox != rec.Mailbox || got.TokenSHA256 != rec.TokenSHA256 || !bytes.Equal(got.SecretEnc, rec.SecretEnc) {
		t.Errorf("GetByID roundtrip mismatch: %+v", got)
	}
	if !got.Active(now) {
		t.Error("fresh key should be active")
	}

	if err := st.TouchLastUsed("kid1", now.Add(time.Minute)); err != nil {
		t.Fatalf("TouchLastUsed: %v", err)
	}
	if touched, _ := st.GetByID("kid1"); touched.LastUsed.IsZero() {
		t.Error("last_used should be recorded after touch")
	}

	if miss, _ := st.GetByID("nope"); miss != nil {
		t.Error("GetByID(missing) should be nil")
	}

	// List must not leak secret material.
	list, err := st.List("")
	if err != nil || len(list) != 1 {
		t.Fatalf("List: %v n=%d", err, len(list))
	}
	if list[0].SecretEnc != nil || list[0].TokenSHA256 != "" {
		t.Error("List must not select secret columns")
	}
	if l, _ := st.List("other@example.com"); len(l) != 0 {
		t.Error("mailbox filter should exclude non-matching keys")
	}

	// Revoke is idempotent and reports existence.
	if ok, _ := st.Revoke("kid1", now); !ok {
		t.Error("Revoke should report the key existed")
	}
	if again, _ := st.Revoke("kid1", now.Add(time.Hour)); !again {
		t.Error("Revoke should stay idempotent-true for an existing key")
	}
	if ok, _ := st.Revoke("ghost", now); ok {
		t.Error("Revoke of a missing key should report false")
	}
	after, _ := st.GetByID("kid1")
	if after.Active(now) {
		t.Error("revoked key must be inactive")
	}
}

func TestRecordActive(t *testing.T) {
	now := time.Unix(1_000_000, 0).UTC()
	expired := Record{Expires: now.Add(-time.Hour)}
	if expired.Active(now) {
		t.Error("expired key must be inactive")
	}
	future := Record{Expires: now.Add(time.Hour)}
	if !future.Active(now) {
		t.Error("not-yet-expired key should be active")
	}
}
