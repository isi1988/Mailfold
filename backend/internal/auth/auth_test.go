package auth

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestLoginSuccessAndValidate(t *testing.T) {
	a := New("admin", "pw", time.Hour)
	sess, err := a.Login("admin", "pw", SessionMeta{IP: "127.0.0.1", UserAgent: "test-agent"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if sess.Token == "" {
		t.Fatal("expected a non-empty token")
	}
	if sess.IP != "127.0.0.1" || sess.UserAgent != "test-agent" {
		t.Errorf("session meta not recorded: %+v", sess)
	}
	got, ok := a.Validate(sess.Token)
	if !ok || got.User != "admin" {
		t.Errorf("Validate failed: ok=%v sess=%+v", ok, got)
	}
	a.Logout(sess.Token)
	if _, ok := a.Validate(sess.Token); ok {
		t.Error("token should be invalid after logout")
	}
}

func TestLoginFailures(t *testing.T) {
	a := New("admin", "pw", time.Hour)
	if _, err := a.Login("admin", "wrong", SessionMeta{}); err != ErrInvalidCredentials {
		t.Errorf("want ErrInvalidCredentials, got %v", err)
	}
	if _, err := a.Login("nobody", "pw", SessionMeta{}); err != ErrInvalidCredentials {
		t.Errorf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestValidateEmptyAndUnknown(t *testing.T) {
	a := New("admin", "pw", time.Hour)
	if _, ok := a.Validate(""); ok {
		t.Error("empty token should be invalid")
	}
	if _, ok := a.Validate("deadbeef"); ok {
		t.Error("unknown token should be invalid")
	}
}

func TestExpiryAndGC(t *testing.T) {
	a := New("admin", "pw", -time.Second) // sessions expire immediately
	sess, err := a.Login("admin", "pw", SessionMeta{})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if _, ok := a.Validate(sess.Token); ok {
		t.Error("expired session should be invalid")
	}

	sess2, _ := a.Login("admin", "pw", SessionMeta{})
	a.GC()
	if _, ok := a.Validate(sess2.Token); ok {
		t.Error("GC should have removed the expired session")
	}
}

func TestSetPasswordHashOverridesPlaintext(t *testing.T) {
	a := New("admin", "old-pw", time.Hour)
	hash, err := bcrypt.GenerateFromPassword([]byte("new-pw"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	a.SetPasswordHash(string(hash))

	if a.CheckPassword("admin", "old-pw") {
		t.Error("plaintext password should no longer work once a hash override is set")
	}
	if !a.CheckPassword("admin", "new-pw") {
		t.Error("the new (hashed) password should work")
	}

	// Clearing the override falls back to the plaintext configured password.
	a.SetPasswordHash("")
	if !a.CheckPassword("admin", "old-pw") {
		t.Error("clearing the override should restore the plaintext password")
	}
}

func TestPendingTwoFactorFlow(t *testing.T) {
	a := New("admin", "pw", time.Hour)
	if !a.CheckPassword("admin", "pw") {
		t.Fatal("password should check out")
	}
	token, err := a.IssuePending()
	if err != nil {
		t.Fatalf("IssuePending: %v", err)
	}
	user, ok := a.ConsumePending(token)
	if !ok || user != "admin" {
		t.Fatalf("ConsumePending: ok=%v user=%q", ok, user)
	}
	// A pending token is single-use.
	if _, ok := a.ConsumePending(token); ok {
		t.Error("pending token should not be redeemable twice")
	}
	if _, ok := a.ConsumePending("bogus"); ok {
		t.Error("unknown pending token should not redeem")
	}

	sess, err := a.MintSession(SessionMeta{IP: "10.0.0.1"})
	if err != nil {
		t.Fatalf("MintSession: %v", err)
	}
	if sess.User != "admin" || sess.IP != "10.0.0.1" {
		t.Errorf("unexpected session: %+v", sess)
	}
}

func TestListRevokeSessions(t *testing.T) {
	a := New("admin", "pw", time.Hour)
	s1, _ := a.Login("admin", "pw", SessionMeta{IP: "1.1.1.1"})
	s2, _ := a.Login("admin", "pw", SessionMeta{IP: "2.2.2.2"})

	list := a.ListSessions(s1.Token)
	if len(list) != 2 {
		t.Fatalf("want 2 sessions, got %d", len(list))
	}
	var sawCurrent bool
	for _, si := range list {
		if si.Current {
			sawCurrent = true
		}
		if si.ID == "" {
			t.Error("session id should not be empty")
		}
	}
	if !sawCurrent {
		t.Error("exactly one session should be marked current")
	}

	// RevokeByID logs out the targeted session only.
	id2 := ""
	for _, si := range list {
		if !si.Current {
			id2 = si.ID
		}
	}
	if !a.RevokeByID(id2) {
		t.Fatal("RevokeByID should find the session")
	}
	if _, ok := a.Validate(s2.Token); ok {
		t.Error("revoked session should be invalid")
	}
	if _, ok := a.Validate(s1.Token); !ok {
		t.Error("other session should remain valid")
	}
	if a.RevokeByID("does-not-exist") {
		t.Error("RevokeByID should report false for an unknown id")
	}

	// RevokeAllExcept clears everything but the given token.
	s3, _ := a.Login("admin", "pw", SessionMeta{})
	n := a.RevokeAllExcept(s1.Token)
	if n != 1 {
		t.Errorf("RevokeAllExcept should have revoked 1 session, got %d", n)
	}
	if _, ok := a.Validate(s3.Token); ok {
		t.Error("s3 should have been revoked")
	}
	if _, ok := a.Validate(s1.Token); !ok {
		t.Error("s1 (the excluded token) should remain valid")
	}
}
