package auth

import (
	"testing"
	"time"
)

func TestLoginSuccessAndValidate(t *testing.T) {
	a := New("admin", "pw", time.Hour)
	sess, err := a.Login("admin", "pw")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if sess.Token == "" {
		t.Fatal("expected a non-empty token")
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
	if _, err := a.Login("admin", "wrong"); err != ErrInvalidCredentials {
		t.Errorf("want ErrInvalidCredentials, got %v", err)
	}
	if _, err := a.Login("nobody", "pw"); err != ErrInvalidCredentials {
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
	sess, err := a.Login("admin", "pw")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if _, ok := a.Validate(sess.Token); ok {
		t.Error("expired session should be invalid")
	}

	sess2, _ := a.Login("admin", "pw")
	a.GC()
	if _, ok := a.Validate(sess2.Token); ok {
		t.Error("GC should have removed the expired session")
	}
}
