package admin

import (
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open("sqlite", t.TempDir()+"/admin.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestGetAccountLazyRow(t *testing.T) {
	st := openTestStore(t)
	acct, err := st.GetAccount("admin")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acct.Username != "admin" || acct.PasswordHash != "" || acct.TOTPEnabled {
		t.Errorf("unexpected zero-value account: %+v", acct)
	}
}

func TestSetPasswordHash(t *testing.T) {
	st := openTestStore(t)
	if err := st.SetPasswordHash("admin", "hash1", time.Now()); err != nil {
		t.Fatalf("SetPasswordHash: %v", err)
	}
	acct, err := st.GetAccount("admin")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acct.PasswordHash != "hash1" {
		t.Errorf("PasswordHash = %q, want hash1", acct.PasswordHash)
	}
	// A second write on an existing row must update, not fail.
	if err := st.SetPasswordHash("admin", "hash2", time.Now()); err != nil {
		t.Fatalf("SetPasswordHash (update): %v", err)
	}
	acct, _ = st.GetAccount("admin")
	if acct.PasswordHash != "hash2" {
		t.Errorf("PasswordHash = %q, want hash2", acct.PasswordHash)
	}
}

func TestSetProfile(t *testing.T) {
	st := openTestStore(t)
	up := ProfileUpdate{DisplayName: "Admin", Email: "admin@example.com", Timezone: "UTC", AvatarURL: "https://example.com/a.png"}
	if err := st.SetProfile("admin", up, time.Now()); err != nil {
		t.Fatalf("SetProfile: %v", err)
	}
	acct, err := st.GetAccount("admin")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acct.DisplayName != up.DisplayName || acct.Email != up.Email || acct.Timezone != up.Timezone || acct.AvatarURL != up.AvatarURL {
		t.Errorf("profile not persisted: %+v", acct)
	}
}

func TestSetTOTP(t *testing.T) {
	st := openTestStore(t)
	enc, nonce := []byte("cipher"), []byte("nonce")
	if err := st.SetTOTP("admin", true, enc, nonce, time.Now()); err != nil {
		t.Fatalf("SetTOTP: %v", err)
	}
	acct, err := st.GetAccount("admin")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if !acct.TOTPEnabled || string(acct.TOTPSecretEnc) != "cipher" || string(acct.TOTPSecretNonce) != "nonce" {
		t.Errorf("TOTP not persisted: %+v", acct)
	}

	if err := st.SetTOTP("admin", false, nil, nil, time.Now()); err != nil {
		t.Fatalf("SetTOTP (disable): %v", err)
	}
	acct, _ = st.GetAccount("admin")
	if acct.TOTPEnabled {
		t.Error("TOTP should be disabled")
	}
}

func TestSetNotifySender(t *testing.T) {
	st := openTestStore(t)
	if err := st.SetNotifySender("admin", "noreply@example.com", []byte("enc"), []byte("nonce"), time.Now()); err != nil {
		t.Fatalf("SetNotifySender: %v", err)
	}
	acct, err := st.GetAccount("admin")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acct.NotifyMailbox != "noreply@example.com" || string(acct.NotifyPasswordEnc) != "enc" {
		t.Errorf("notify sender not persisted: %+v", acct)
	}
}

func TestRecoveryCodeLifecycle(t *testing.T) {
	st := openTestStore(t)
	hashes := []string{"h1", "h2", "h3"}
	if err := st.ReplaceRecoveryCodes("admin", hashes); err != nil {
		t.Fatalf("ReplaceRecoveryCodes: %v", err)
	}
	n, err := st.RemainingRecoveryCodes("admin")
	if err != nil {
		t.Fatalf("RemainingRecoveryCodes: %v", err)
	}
	if n != 3 {
		t.Fatalf("want 3 remaining, got %d", n)
	}

	ok, err := st.ConsumeRecoveryCode("admin", "h1", time.Now())
	if err != nil || !ok {
		t.Fatalf("ConsumeRecoveryCode: ok=%v err=%v", ok, err)
	}
	// A code can only be consumed once.
	ok, err = st.ConsumeRecoveryCode("admin", "h1", time.Now())
	if err != nil || ok {
		t.Fatalf("ConsumeRecoveryCode should not re-consume: ok=%v err=%v", ok, err)
	}
	ok, err = st.ConsumeRecoveryCode("admin", "does-not-exist", time.Now())
	if err != nil || ok {
		t.Fatalf("ConsumeRecoveryCode should reject an unknown hash: ok=%v err=%v", ok, err)
	}

	n, _ = st.RemainingRecoveryCodes("admin")
	if n != 2 {
		t.Errorf("want 2 remaining after one consumed, got %d", n)
	}

	// Replacing invalidates the old set entirely.
	if err := st.ReplaceRecoveryCodes("admin", []string{"h4"}); err != nil {
		t.Fatalf("ReplaceRecoveryCodes (2nd): %v", err)
	}
	ok, _ = st.ConsumeRecoveryCode("admin", "h2", time.Now())
	if ok {
		t.Error("old recovery codes should be gone after replacement")
	}
	n, _ = st.RemainingRecoveryCodes("admin")
	if n != 1 {
		t.Errorf("want 1 remaining after replacement, got %d", n)
	}
}

func TestResetTokenLifecycle(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()
	if err := st.CreateResetToken("tok-hash", "admin", now.Add(time.Hour)); err != nil {
		t.Fatalf("CreateResetToken: %v", err)
	}

	user, ok, err := st.ConsumeResetToken("tok-hash", now)
	if err != nil || !ok || user != "admin" {
		t.Fatalf("ConsumeResetToken: user=%q ok=%v err=%v", user, ok, err)
	}
	// Single-use.
	_, ok, err = st.ConsumeResetToken("tok-hash", now)
	if err != nil || ok {
		t.Fatalf("ConsumeResetToken should not redeem twice: ok=%v err=%v", ok, err)
	}

	if err := st.CreateResetToken("expired-hash", "admin", now.Add(-time.Minute)); err != nil {
		t.Fatalf("CreateResetToken (expired): %v", err)
	}
	_, ok, err = st.ConsumeResetToken("expired-hash", now)
	if err != nil || ok {
		t.Fatalf("ConsumeResetToken should reject an expired token: ok=%v err=%v", ok, err)
	}

	_, ok, err = st.ConsumeResetToken("unknown-hash", now)
	if err != nil || ok {
		t.Fatalf("ConsumeResetToken should reject an unknown token: ok=%v err=%v", ok, err)
	}
}

func TestKnownDeviceLifecycle(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()

	known, err := st.IsKnownDevice("admin", "fp-1")
	if err != nil {
		t.Fatalf("IsKnownDevice: %v", err)
	}
	if known {
		t.Error("a fingerprint never recorded should not be known")
	}

	if err := st.RecordDevice("admin", "fp-1", now); err != nil {
		t.Fatalf("RecordDevice: %v", err)
	}
	known, err = st.IsKnownDevice("admin", "fp-1")
	if err != nil {
		t.Fatalf("IsKnownDevice: %v", err)
	}
	if !known {
		t.Error("fingerprint should be known after RecordDevice")
	}

	// A different fingerprint for the same user is still unknown.
	known, err = st.IsKnownDevice("admin", "fp-2")
	if err != nil {
		t.Fatalf("IsKnownDevice: %v", err)
	}
	if known {
		t.Error("a different fingerprint should not be known")
	}

	// Recording the same fingerprint again (e.g. a later sign-in) must not
	// error — it should update last_seen in place rather than conflict.
	if err := st.RecordDevice("admin", "fp-1", now.Add(time.Hour)); err != nil {
		t.Fatalf("RecordDevice (repeat): %v", err)
	}
}

func TestWebAuthnCredentialLifecycle(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()

	creds, err := st.ListWebAuthnCredentials("admin")
	if err != nil {
		t.Fatalf("ListWebAuthnCredentials: %v", err)
	}
	if len(creds) != 0 {
		t.Fatalf("want 0 credentials before enrollment, got %d", len(creds))
	}

	c := WebAuthnCredential{
		Username:     "admin",
		CredentialID: []byte{1, 2, 3},
		PublicKey:    []byte{4, 5, 6},
		SignCount:    0,
		Transports:   "internal,hybrid",
		Name:         "MacBook Touch ID",
	}
	if err := st.AddWebAuthnCredential(c, now); err != nil {
		t.Fatalf("AddWebAuthnCredential: %v", err)
	}

	creds, err = st.ListWebAuthnCredentials("admin")
	if err != nil {
		t.Fatalf("ListWebAuthnCredentials: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("want 1 credential after enrollment, got %d", len(creds))
	}
	got := creds[0]
	if got.Name != c.Name || got.Transports != c.Transports || got.SignCount != 0 {
		t.Errorf("unexpected stored credential: %+v", got)
	}
	if string(got.CredentialID) != string(c.CredentialID) || string(got.PublicKey) != string(c.PublicKey) {
		t.Errorf("credential id/public key not round-tripped: %+v", got)
	}

	// A credential enrolled for a different user must not show up here.
	other, err := st.ListWebAuthnCredentials("someone-else")
	if err != nil {
		t.Fatalf("ListWebAuthnCredentials(someone-else): %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("want 0 credentials for a different user, got %d", len(other))
	}

	if err := st.UpdateWebAuthnSignCount(c.CredentialID, 7); err != nil {
		t.Fatalf("UpdateWebAuthnSignCount: %v", err)
	}
	creds, _ = st.ListWebAuthnCredentials("admin")
	if creds[0].SignCount != 7 {
		t.Errorf("want sign_count 7 after update, got %d", creds[0].SignCount)
	}

	if err := st.DeleteWebAuthnCredential("admin", got.ID); err != nil {
		t.Fatalf("DeleteWebAuthnCredential: %v", err)
	}
	creds, _ = st.ListWebAuthnCredentials("admin")
	if len(creds) != 0 {
		t.Fatalf("want 0 credentials after delete, got %d", len(creds))
	}
}
