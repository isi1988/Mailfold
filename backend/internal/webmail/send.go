package webmail

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// OutgoingMessage is a message composed by the user to be submitted for delivery.
type OutgoingMessage struct {
	To      []string `json:"to"`
	Cc      []string `json:"cc"`
	Bcc     []string `json:"bcc"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
	HTML    string   `json:"html"`
}

// recipients returns every envelope recipient (To + Cc + Bcc).
func (m *OutgoingMessage) recipients() []string {
	out := make([]string, 0, len(m.To)+len(m.Cc)+len(m.Bcc))
	out = append(out, m.To...)
	out = append(out, m.Cc...)
	out = append(out, m.Bcc...)
	return out
}

// Send composes msg and submits it via SMTP authenticated as the given user.
//
// Submission uses opportunistic STARTTLS: it connects in plaintext and upgrades
// when the server advertises STARTTLS (the standard on submission port 587).
// TLS certificate verification honours the client's insecure flag.
func (c *Client) Send(email, password string, msg *OutgoingMessage) error {
	if len(msg.recipients()) == 0 {
		return fmt.Errorf("message has no recipients")
	}
	raw, err := renderMessage(email, msg)
	if err != nil {
		return err
	}

	host, _, _ := net.SplitHostPort(c.smtpAddr)
	conn, err := net.Dial("tcp", c.smtpAddr)
	if err != nil {
		return err
	}
	sc := smtp.NewClient(conn)
	if ok, _ := sc.Extension("STARTTLS"); ok {
		_ = sc.Close()
		if conn, err = net.Dial("tcp", c.smtpAddr); err != nil {
			return err
		}
		sc, err = smtp.NewClientStartTLS(conn, &tls.Config{ServerName: host, InsecureSkipVerify: c.insecureTLS}) //nolint:gosec // insecure is an opt-in dev flag
		if err != nil {
			return err
		}
	}
	defer func() { _ = sc.Close() }()

	if err := sc.Auth(sasl.NewPlainClient("", email, password)); err != nil {
		return err
	}
	if err := sc.SendMail(email, msg.recipients(), bytes.NewReader(raw)); err != nil {
		return err
	}
	return sc.Quit()
}

// renderMessage builds an RFC 5322 message with a text and/or HTML body.
func renderMessage(from string, msg *OutgoingMessage) ([]byte, error) {
	var buf bytes.Buffer
	var h mail.Header
	h.SetDate(time.Now())
	h.SetAddressList("From", []*mail.Address{{Address: from}})
	setAddrHeader(&h, "To", msg.To)
	setAddrHeader(&h, "Cc", msg.Cc)
	h.SetSubject(msg.Subject)

	mw, err := mail.CreateWriter(&buf, h)
	if err != nil {
		return nil, err
	}
	tw, err := mw.CreateInline()
	if err != nil {
		return nil, err
	}
	// Always include a text/plain part (even if empty) so the message has a body.
	if err := writeInlinePart(tw, "text/plain", msg.Text); err != nil {
		return nil, err
	}
	if msg.HTML != "" {
		if err := writeInlinePart(tw, "text/html", msg.HTML); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeInlinePart writes one inline body part of the given content type.
func writeInlinePart(tw *mail.InlineWriter, contentType, body string) error {
	var h mail.InlineHeader
	h.SetContentType(contentType, map[string]string{"charset": "utf-8"})
	w, err := tw.CreatePart(h)
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(body)); err != nil {
		return err
	}
	return w.Close()
}

// setAddrHeader sets an address-list header when the list is non-empty.
func setAddrHeader(h *mail.Header, key string, addrs []string) {
	if len(addrs) == 0 {
		return
	}
	list := make([]*mail.Address, 0, len(addrs))
	for _, a := range addrs {
		list = append(list, &mail.Address{Address: a})
	}
	h.SetAddressList(key, list)
}
