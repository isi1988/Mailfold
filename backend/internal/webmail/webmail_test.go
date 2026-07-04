package webmail

import (
	"bytes"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/backend/memory"
	"github.com/emersion/go-imap/server"
	"github.com/emersion/go-message/mail"
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

func TestSearch(t *testing.T) {
	c := NewClient(startIMAP(t), "", false, false)
	if _, err := c.Search("username", "password", "INBOX", "message"); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if _, err := NewClient("127.0.0.1:1", "", false, false).Search("u", "p", "INBOX", "x"); err == nil {
		t.Error("Search should fail when the server is unreachable")
	}
}

func TestAttachmentErrorPaths(t *testing.T) {
	c := NewClient(startIMAP(t), "", false, false)
	msgs, err := c.Messages("username", "password", "INBOX", 5)
	if err != nil || len(msgs) == 0 {
		t.Fatalf("Messages: err=%v n=%d", err, len(msgs))
	}
	// The sample message has no attachment.
	if _, _, _, err := c.Attachment("username", "password", "INBOX", msgs[0].UID, 0); err == nil {
		t.Error("expected an attachment-not-found error")
	}
	if _, _, _, err := NewClient("127.0.0.1:1", "", false, false).Attachment("u", "p", "INBOX", 1, 0); err == nil {
		t.Error("Attachment should fail when unreachable")
	}
}

func TestNthAttachment(t *testing.T) {
	raw := buildMessageWithAttachment(t)
	name, ct, data, err := nthAttachment(bytes.NewReader(raw), 0)
	if err != nil {
		t.Fatalf("nthAttachment: %v", err)
	}
	if name != "hello.txt" || !strings.HasPrefix(ct, "text/plain") || string(data) != "attached content" {
		t.Errorf("got name=%q ct=%q data=%q", name, ct, string(data))
	}
	if _, _, _, err := nthAttachment(bytes.NewReader(raw), 5); err == nil {
		t.Error("expected not-found for an out-of-range index")
	}
}

func buildMessageWithAttachment(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	var h mail.Header
	h.SetSubject("with attachment")
	h.SetAddressList("From", []*mail.Address{{Address: "a@b.c"}})
	mw, err := mail.CreateWriter(&buf, h)
	if err != nil {
		t.Fatal(err)
	}
	tw, _ := mw.CreateInline()
	var ih mail.InlineHeader
	ih.SetContentType("text/plain", map[string]string{"charset": "utf-8"})
	iw, _ := tw.CreatePart(ih)
	_, _ = iw.Write([]byte("body"))
	_ = iw.Close()
	_ = tw.Close()

	var ah mail.AttachmentHeader
	ah.SetContentType("text/plain", map[string]string{"charset": "utf-8"})
	ah.SetFilename("hello.txt")
	aw, _ := mw.CreateAttachment(ah)
	_, _ = aw.Write([]byte("attached content"))
	_ = aw.Close()
	_ = mw.Close()
	return buf.Bytes()
}

func TestActions(t *testing.T) {
	c := NewClient(startIMAP(t), "", false, false)
	const u, p = "username", "password"

	msgs, err := c.Messages(u, p, "INBOX", 10)
	if err != nil || len(msgs) == 0 {
		t.Fatalf("Messages: err=%v n=%d", err, len(msgs))
	}
	uid := msgs[0].UID

	if err := c.SetFlag(u, p, "INBOX", uid, "flagged", true); err != nil {
		t.Fatalf("SetFlag set: %v", err)
	}
	if err := c.SetFlag(u, p, "INBOX", uid, "flagged", false); err != nil {
		t.Fatalf("SetFlag unset: %v", err)
	}
	if err := c.SetFlag(u, p, "INBOX", uid, "bogus", true); err == nil {
		t.Error("SetFlag with an unknown flag should fail")
	}

	if err := c.CreateFolder(u, p, "Archive"); err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	if err := c.Move(u, p, "INBOX", uid, "Archive"); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if after, _ := c.Messages(u, p, "INBOX", 10); len(after) != 0 {
		t.Errorf("INBOX should be empty after move, has %d", len(after))
	}

	arch, err := c.Messages(u, p, "Archive", 10)
	if err != nil || len(arch) == 0 {
		t.Fatalf("Archive messages: err=%v n=%d", err, len(arch))
	}
	if err := c.Delete(u, p, "Archive", arch[0].UID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if final, _ := c.Messages(u, p, "Archive", 10); len(final) != 0 {
		t.Errorf("Archive should be empty after delete, has %d", len(final))
	}
}

func TestActionErrorPaths(t *testing.T) {
	c := NewClient("127.0.0.1:1", "", false, false) // unreachable
	if err := c.SetFlag("u", "p", "INBOX", 1, "seen", true); err == nil {
		t.Error("SetFlag should fail when unreachable")
	}
	if err := c.Move("u", "p", "INBOX", 1, "X"); err == nil {
		t.Error("Move should fail when unreachable")
	}
	if err := c.Delete("u", "p", "INBOX", 1); err == nil {
		t.Error("Delete should fail when unreachable")
	}
	if err := c.CreateFolder("u", "p", "X"); err == nil {
		t.Error("CreateFolder should fail when unreachable")
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

func TestSessionsTake(t *testing.T) {
	s := NewSessions(time.Hour)
	token, _, err := s.Create("u@example.com", "pw")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	cred, ok := s.Take(token)
	if !ok || cred.Email != "u@example.com" || cred.Password != "pw" {
		t.Fatalf("Take failed: %v %+v", ok, cred)
	}
	// Single-use: a second Take of the same token must fail.
	if _, ok := s.Take(token); ok {
		t.Error("Take should be single-use")
	}
	if _, ok := s.Take(""); ok {
		t.Error("empty token must be invalid")
	}
}

func TestSessionsTakeExpired(t *testing.T) {
	s := NewSessions(-time.Second) // expire immediately
	token, _, _ := s.Create("a", "b")
	if _, ok := s.Take(token); ok {
		t.Error("Take should reject an expired token")
	}
}

// TestSessionsPeekSurvivesAWrongCode is the regression test for the bug where
// any failed second-factor attempt permanently stranded a pending webmail
// login: Peek must not consume the token, so it can be verified again.
func TestSessionsPeekSurvivesAWrongCode(t *testing.T) {
	s := NewSessions(time.Hour)
	token, _, err := s.Create("u@example.com", "pw")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Simulate a wrong code: the handler calls Peek, the code check fails,
	// and it never calls Delete.
	if _, ok := s.Peek(token); !ok {
		t.Fatal("first (simulated wrong-code) peek should still succeed")
	}
	cred, ok := s.Peek(token)
	if !ok || cred.Email != "u@example.com" || cred.Password != "pw" {
		t.Fatalf("pending token should survive a prior failed code check: ok=%v %+v", ok, cred)
	}
	// Only an explicit Delete (called once a code actually verifies) removes it.
	s.Delete(token)
	if _, ok := s.Peek(token); ok {
		t.Error("token should be gone after Delete")
	}
}

func TestSessionsPeekAttemptCap(t *testing.T) {
	s := NewSessions(time.Hour)
	token, _, err := s.Create("u@example.com", "pw")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	for i := 0; i < maxPendingAttempts; i++ {
		if _, ok := s.Peek(token); !ok {
			t.Fatalf("attempt %d should still be within budget", i+1)
		}
	}
	if _, ok := s.Peek(token); ok {
		t.Error("token should be invalidated once the attempt budget is exceeded")
	}
}

func TestSessionsPeekExpiredAndEmpty(t *testing.T) {
	s := NewSessions(-time.Second) // expire immediately
	token, _, _ := s.Create("a", "b")
	if _, ok := s.Peek(token); ok {
		t.Error("Peek should reject an expired token")
	}
	if _, ok := s.Peek(""); ok {
		t.Error("empty token must be invalid")
	}
	if _, ok := s.Peek("bogus"); ok {
		t.Error("unknown token must be invalid")
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

func TestCheckSince(t *testing.T) {
	c := NewClient(startIMAP(t), "", false, false)

	// Baseline: no new messages reported, but a positive high-water UID for the
	// sample INBOX the memory backend ships with.
	msgs, base, err := c.CheckSince("username", "password", 0)
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("baseline should report no new messages, got %d", len(msgs))
	}
	if base == 0 {
		t.Fatal("baseline UID should be > 0 for the sample INBOX")
	}

	// Nothing new since the baseline.
	if m, u, err := c.CheckSince("username", "password", base); err != nil || len(m) != 0 || u != base {
		t.Fatalf("no-new: err=%v n=%d uid=%d (want 0 new, uid=%d)", err, len(m), u, base)
	}

	// Asking for anything above base-1 must surface the newest message.
	fresh, u, err := c.CheckSince("username", "password", base-1)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(fresh) == 0 {
		t.Fatal("expected to detect the newest message")
	}
	if u != base {
		t.Fatalf("high-water UID = %d, want %d", u, base)
	}

	// A dial failure propagates.
	bad := NewClient("127.0.0.1:1", "", false, false)
	if _, _, err := bad.CheckSince("u", "p", 0); err == nil {
		t.Error("CheckSince should fail when the server is unreachable")
	}
}

func TestSaveToSent(t *testing.T) {
	c := NewClient(startIMAP(t), "", false, false)
	msg := &OutgoingMessage{To: []string{"x@example.com"}, Subject: "hi", Text: "body"}

	// The sample backend has no Sent folder; SaveToSent must create it and append.
	if err := c.SaveToSent("username", "password", msg); err != nil {
		t.Fatalf("SaveToSent: %v", err)
	}
	msgs, err := c.Messages("username", "password", "Sent", 10)
	if err != nil {
		t.Fatalf("read Sent: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("expected the sent copy to appear in Sent")
	}

	// A dial failure propagates.
	bad := NewClient("127.0.0.1:1", "", false, false)
	if err := bad.SaveToSent("u", "p", msg); err == nil {
		t.Error("SaveToSent should fail when the server is unreachable")
	}
}

func TestMessagesPreview(t *testing.T) {
	c := NewClient(startIMAP(t), "", false, false)
	msgs, err := c.Messages("username", "password", "INBOX", 10)
	if err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("expected at least one message")
	}
	// The sample INBOX message has a body, so a non-empty single-line preview
	// should be derived.
	if strings.TrimSpace(msgs[0].Preview) == "" {
		t.Errorf("expected a body preview, got empty (subject=%q)", msgs[0].Subject)
	}
	if strings.Contains(msgs[0].Preview, "\n") {
		t.Errorf("preview should be a single line, got %q", msgs[0].Preview)
	}
}
