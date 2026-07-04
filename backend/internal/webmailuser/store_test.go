package webmailuser

import (
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open("sqlite", t.TempDir()+"/webmailuser.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestGetAccountLazyRow(t *testing.T) {
	st := openTestStore(t)
	acct, err := st.GetAccount("noreply@родоскоп.рф")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acct.Email != "noreply@родоскоп.рф" || acct.Signature != "" || acct.TOTPEnabled {
		t.Errorf("unexpected zero-value account: %+v", acct)
	}
}

func TestSetSignature(t *testing.T) {
	st := openTestStore(t)
	if err := st.SetSignature("a@example.com", "Best,\nA", time.Now()); err != nil {
		t.Fatalf("SetSignature: %v", err)
	}
	acct, err := st.GetAccount("a@example.com")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acct.Signature != "Best,\nA" {
		t.Errorf("Signature = %q, want %q", acct.Signature, "Best,\nA")
	}
	// Replacing it overwrites, it does not append.
	if err := st.SetSignature("a@example.com", "New sig", time.Now()); err != nil {
		t.Fatalf("SetSignature (replace): %v", err)
	}
	acct, err = st.GetAccount("a@example.com")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acct.Signature != "New sig" {
		t.Errorf("Signature after replace = %q, want %q", acct.Signature, "New sig")
	}
}

func TestSignatureIsPerMailbox(t *testing.T) {
	st := openTestStore(t)
	if err := st.SetSignature("a@example.com", "sig A", time.Now()); err != nil {
		t.Fatalf("SetSignature a: %v", err)
	}
	if err := st.SetSignature("b@example.com", "sig B", time.Now()); err != nil {
		t.Fatalf("SetSignature b: %v", err)
	}
	a, _ := st.GetAccount("a@example.com")
	b, _ := st.GetAccount("b@example.com")
	if a.Signature != "sig A" || b.Signature != "sig B" {
		t.Errorf("signatures leaked across mailboxes: a=%q b=%q", a.Signature, b.Signature)
	}
}

func TestSetTOTP(t *testing.T) {
	st := openTestStore(t)
	enc, nonce := []byte("enc"), []byte("nonce")
	if err := st.SetTOTP("a@example.com", true, enc, nonce, time.Now()); err != nil {
		t.Fatalf("SetTOTP: %v", err)
	}
	acct, err := st.GetAccount("a@example.com")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if !acct.TOTPEnabled || string(acct.TOTPSecretEnc) != "enc" || string(acct.TOTPSecretNonce) != "nonce" {
		t.Errorf("unexpected TOTP state: %+v", acct)
	}

	if err := st.SetTOTP("a@example.com", false, nil, nil, time.Now()); err != nil {
		t.Fatalf("SetTOTP (disable): %v", err)
	}
	acct, err = st.GetAccount("a@example.com")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acct.TOTPEnabled || len(acct.TOTPSecretEnc) != 0 {
		t.Errorf("TOTP should be disabled and cleared: %+v", acct)
	}
}

func TestRecoveryCodes(t *testing.T) {
	st := openTestStore(t)
	if err := st.ReplaceRecoveryCodes("a@example.com", []string{"h1", "h2", "h3"}); err != nil {
		t.Fatalf("ReplaceRecoveryCodes: %v", err)
	}
	n, err := st.RemainingRecoveryCodes("a@example.com")
	if err != nil {
		t.Fatalf("RemainingRecoveryCodes: %v", err)
	}
	if n != 3 {
		t.Errorf("RemainingRecoveryCodes = %d, want 3", n)
	}

	ok, err := st.ConsumeRecoveryCode("a@example.com", "h1", time.Now())
	if err != nil || !ok {
		t.Fatalf("ConsumeRecoveryCode(h1) = %v, %v, want true, nil", ok, err)
	}
	// A used code cannot be consumed again.
	ok, err = st.ConsumeRecoveryCode("a@example.com", "h1", time.Now())
	if err != nil || ok {
		t.Fatalf("ConsumeRecoveryCode(h1) second time = %v, %v, want false, nil", ok, err)
	}
	// An unknown code fails too.
	ok, err = st.ConsumeRecoveryCode("a@example.com", "unknown", time.Now())
	if err != nil || ok {
		t.Fatalf("ConsumeRecoveryCode(unknown) = %v, %v, want false, nil", ok, err)
	}

	n, err = st.RemainingRecoveryCodes("a@example.com")
	if err != nil {
		t.Fatalf("RemainingRecoveryCodes: %v", err)
	}
	if n != 2 {
		t.Errorf("RemainingRecoveryCodes after consuming one = %d, want 2", n)
	}

	// Replacing wipes the old set entirely, including used codes.
	if err := st.ReplaceRecoveryCodes("a@example.com", []string{"h4"}); err != nil {
		t.Fatalf("ReplaceRecoveryCodes (replace): %v", err)
	}
	n, err = st.RemainingRecoveryCodes("a@example.com")
	if err != nil {
		t.Fatalf("RemainingRecoveryCodes: %v", err)
	}
	if n != 1 {
		t.Errorf("RemainingRecoveryCodes after replace = %d, want 1", n)
	}
}

func TestRecoveryCodesArePerMailbox(t *testing.T) {
	st := openTestStore(t)
	if err := st.ReplaceRecoveryCodes("a@example.com", []string{"shared-hash"}); err != nil {
		t.Fatalf("ReplaceRecoveryCodes a: %v", err)
	}
	// b@example.com never got this code, so it must not be consumable there.
	ok, err := st.ConsumeRecoveryCode("b@example.com", "shared-hash", time.Now())
	if err != nil || ok {
		t.Fatalf("ConsumeRecoveryCode for a different mailbox = %v, %v, want false, nil", ok, err)
	}
}
