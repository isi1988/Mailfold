package webmail

import (
	"net"
	"testing"
	"time"

	"github.com/emersion/go-imap/backend/memory"
	"github.com/emersion/go-imap/server"
)

// startIMAP launches an in-memory IMAP server and returns its address. The
// memory backend ships with a user "username"/"password" and a sample INBOX.
func startIMAP(t *testing.T) string {
	t.Helper()
	s := server.New(memory.New())
	s.AllowInsecureAuth = true
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = s.Serve(ln) }()
	t.Cleanup(func() { _ = s.Close() })
	return ln.Addr().String()
}

func TestClientReadFlow(t *testing.T) {
	c := NewClient(startIMAP(t), "", false, false)

	if err := c.Verify("username", "password"); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if err := c.Verify("username", "wrong"); err == nil {
		t.Error("Verify should fail with a wrong password")
	}

	folders, err := c.Folders("username", "password")
	if err != nil || len(folders) == 0 {
		t.Fatalf("Folders: err=%v n=%d", err, len(folders))
	}

	msgs, err := c.Messages("username", "password", "INBOX", 10)
	if err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("expected at least one message in INBOX")
	}

	uid := msgs[0].UID
	msg, err := c.Message("username", "password", "INBOX", uid)
	if err != nil {
		t.Fatalf("Message: %v", err)
	}
	if msg.UID != uid {
		t.Errorf("Message UID = %d, want %d", msg.UID, uid)
	}
}

func TestConfigured(t *testing.T) {
	if NewClient("", "", true, false).Configured() {
		t.Error("empty IMAP address should not be configured")
	}
	if !NewClient("host:993", "", true, false).Configured() {
		t.Error("a set IMAP address should be configured")
	}
}

func TestClientErrorPaths(t *testing.T) {
	c := NewClient("127.0.0.1:1", "", false, false) // nothing listening
	if err := c.Verify("u", "p"); err == nil {
		t.Error("Verify should fail when the server is unreachable")
	}
	if _, err := c.Folders("u", "p"); err == nil {
		t.Error("Folders should fail when unreachable")
	}
	if _, err := c.Messages("u", "p", "INBOX", 10); err == nil {
		t.Error("Messages should fail when unreachable")
	}
	if _, err := c.Message("u", "p", "INBOX", 1); err == nil {
		t.Error("Message should fail when unreachable")
	}
}

func TestSessions(t *testing.T) {
	s := NewSessions(time.Hour)
	token, _, err := s.Create("u@example.com", "pw")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	cred, ok := s.Get(token)
	if !ok || cred.Email != "u@example.com" || cred.Password != "pw" {
		t.Fatalf("Get failed: %v %+v", ok, cred)
	}
	s.Delete(token)
	if _, ok := s.Get(token); ok {
		t.Error("session should be gone after Delete")
	}
	if _, ok := s.Get(""); ok {
		t.Error("empty token must be invalid")
	}
}

func TestSessionExpiryAndGC(t *testing.T) {
	s := NewSessions(-time.Second) // expire immediately
	token, _, _ := s.Create("a", "b")
	if _, ok := s.Get(token); ok {
		t.Error("expired session should be invalid")
	}
	token2, _, _ := s.Create("c", "d")
	s.GC()
	if _, ok := s.Get(token2); ok {
		t.Error("GC should have removed the expired session")
	}
}
