package webmail

import (
	"io"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

type smtpBackend struct {
	mu    sync.Mutex
	count int
}

func (b *smtpBackend) NewSession(_ *smtp.Conn) (smtp.Session, error) {
	return &smtpSession{b: b}, nil
}

type smtpSession struct{ b *smtpBackend }

func (s *smtpSession) AuthMechanisms() []string { return []string{sasl.Plain} }

func (s *smtpSession) Auth(string) (sasl.Server, error) {
	return sasl.NewPlainServer(func(_, _, _ string) error { return nil }), nil
}

func (s *smtpSession) Mail(string, *smtp.MailOptions) error { return nil }
func (s *smtpSession) Rcpt(string, *smtp.RcptOptions) error { return nil }

func (s *smtpSession) Data(r io.Reader) error {
	_, _ = io.Copy(io.Discard, r)
	s.b.mu.Lock()
	s.b.count++
	s.b.mu.Unlock()
	return nil
}

func (s *smtpSession) Reset()        {}
func (s *smtpSession) Logout() error { return nil }

func startSMTP(t *testing.T) (string, *smtpBackend) {
	t.Helper()
	be := &smtpBackend{}
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

func TestSend(t *testing.T) {
	addr, be := startSMTP(t)
	c := NewClient("", addr, false, false)

	err := c.Send("from@example.com", "pw", &OutgoingMessage{
		To:      []string{"to@example.com"},
		Subject: "Hi",
		Text:    "hello",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	be.mu.Lock()
	got := be.count
	be.mu.Unlock()
	if got != 1 {
		t.Errorf("server received %d messages, want 1", got)
	}

	if err := c.Send("from@example.com", "pw", &OutgoingMessage{Subject: "no rcpt"}); err == nil {
		t.Error("expected an error when there are no recipients")
	}
	if err := NewClient("", "127.0.0.1:1", false, false).Send("f", "p", &OutgoingMessage{To: []string{"a@b"}}); err == nil {
		t.Error("expected a dial error against an unreachable server")
	}
}

func TestRenderMessage(t *testing.T) {
	raw, err := renderMessage("from@example.com", &OutgoingMessage{
		To:      []string{"to@example.com"},
		Cc:      []string{"cc@example.com"},
		Subject: "Subject Line",
		Text:    "plain body",
		HTML:    "<b>html body</b>",
	})
	if err != nil {
		t.Fatalf("renderMessage: %v", err)
	}
	s := string(raw)
	for _, want := range []string{"From:", "To:", "Cc:", "Subject: Subject Line", "plain body", "html body"} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered message missing %q", want)
		}
	}
}
