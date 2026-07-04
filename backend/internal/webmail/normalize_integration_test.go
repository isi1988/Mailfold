package webmail

import (
	"errors"
	"io"
	"net"
	"testing"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-imap/server"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// idnIMAPBackend is a minimal IMAP backend whose one registered user is keyed
// by a punycode address, the way mailcow's dovecot only ever recognises an
// internationalized domain in its ASCII-compatible form (confirmed live
// against a production mailcow). It exists to prove Client.dial normalizes
// before calling Login, not just that normalizeAddress computes the right
// string in isolation.
type idnIMAPBackend struct {
	username, password string
}

func (b *idnIMAPBackend) Login(_ *imap.ConnInfo, username, password string) (backend.User, error) {
	if username != b.username || password != b.password {
		return nil, errors.New("invalid credentials")
	}
	return &idnIMAPUser{username: username}, nil
}

type idnIMAPUser struct{ username string }

func (u *idnIMAPUser) Username() string                              { return u.username }
func (u *idnIMAPUser) ListMailboxes(bool) ([]backend.Mailbox, error) { return nil, nil }
func (u *idnIMAPUser) GetMailbox(string) (backend.Mailbox, error) {
	return nil, backend.ErrNoSuchMailbox
}
func (u *idnIMAPUser) CreateMailbox(string) error         { return errors.New("not supported") }
func (u *idnIMAPUser) DeleteMailbox(string) error         { return errors.New("not supported") }
func (u *idnIMAPUser) RenameMailbox(string, string) error { return errors.New("not supported") }
func (u *idnIMAPUser) Logout() error                      { return nil }

func startIDNBackedIMAP(t *testing.T, punycodeUser, password string) string {
	t.Helper()
	be := &idnIMAPBackend{username: punycodeUser, password: password}
	s := server.New(be)
	s.AllowInsecureAuth = true
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = s.Serve(ln) }()
	t.Cleanup(func() { _ = s.Close() })
	return ln.Addr().String()
}

func TestDialNormalizesCyrillicDomainBeforeLogin(t *testing.T) {
	const punycodeUser = "noreply@xn--d1amkbbgbl.xn--p1ai"
	const password = "TempBulkTest2026!"
	addr := startIDNBackedIMAP(t, punycodeUser, password)
	c := NewClient(addr, "", false, false)

	// The account only exists under its punycode name; logging in with the
	// human-typed Cyrillic form must still succeed because dial() normalizes
	// it first.
	if err := c.Verify("noreply@родоскоп.рф", password); err != nil {
		t.Fatalf("Verify with the Cyrillic form should succeed via normalization: %v", err)
	}
	// The punycode form itself must keep working (idempotent).
	if err := c.Verify(punycodeUser, password); err != nil {
		t.Fatalf("Verify with the already-punycode form should still succeed: %v", err)
	}
	// A genuinely wrong local part must still fail — normalization must not
	// paper over real credential mismatches.
	if err := c.Verify("wrong@родоскоп.рф", password); err == nil {
		t.Error("Verify with a wrong local part should fail even though the domain normalizes correctly")
	}
}

// capturingSMTPBackend records the SMTP AUTH username and MAIL FROM address it
// received, so a test can assert the server actually saw the punycode form —
// not just that normalizeAddress returns the right string in isolation.
type capturingSMTPBackend struct {
	authUser string
	mailFrom string
}

func (b *capturingSMTPBackend) NewSession(_ *smtp.Conn) (smtp.Session, error) {
	return &capturingSMTPSession2{b: b}, nil
}

type capturingSMTPSession2 struct{ b *capturingSMTPBackend }

func (s *capturingSMTPSession2) AuthMechanisms() []string { return []string{sasl.Plain} }
func (s *capturingSMTPSession2) Auth(string) (sasl.Server, error) {
	return sasl.NewPlainServer(func(_, username, _ string) error {
		s.b.authUser = username
		return nil
	}), nil
}
func (s *capturingSMTPSession2) Mail(from string, _ *smtp.MailOptions) error {
	s.b.mailFrom = from
	return nil
}
func (s *capturingSMTPSession2) Rcpt(string, *smtp.RcptOptions) error { return nil }
func (s *capturingSMTPSession2) Data(r io.Reader) error {
	_, _ = io.Copy(io.Discard, r)
	return nil
}
func (s *capturingSMTPSession2) Reset()        {}
func (s *capturingSMTPSession2) Logout() error { return nil }

func startCapturingSMTP(t *testing.T) (string, *capturingSMTPBackend) {
	t.Helper()
	be := &capturingSMTPBackend{}
	srv := smtp.NewServer(be)
	srv.AllowInsecureAuth = true
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln.Addr().String(), be
}

func TestSendNormalizesCyrillicDomainInAuthAndEnvelope(t *testing.T) {
	addr, be := startCapturingSMTP(t)
	c := NewClient("", addr, false, false)

	err := c.Send("noreply@родоскоп.рф", "pw", &OutgoingMessage{
		To:      []string{"to@example.com"},
		Subject: "Hi",
		Text:    "hello",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	const wantASCII = "noreply@xn--d1amkbbgbl.xn--p1ai"
	if be.authUser != wantASCII {
		t.Errorf("SMTP AUTH username = %q, want %q", be.authUser, wantASCII)
	}
	if be.mailFrom != wantASCII {
		t.Errorf("MAIL FROM = %q, want %q", be.mailFrom, wantASCII)
	}
}

func TestSendStillWorksWithPlainASCIIAddress(t *testing.T) {
	addr, be := startCapturingSMTP(t)
	c := NewClient("", addr, false, false)

	if err := c.Send("from@example.com", "pw", &OutgoingMessage{To: []string{"to@example.com"}, Subject: "s", Text: "t"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if be.mailFrom != "from@example.com" {
		t.Errorf("MAIL FROM = %q, want unchanged ascii address", be.mailFrom)
	}
}
