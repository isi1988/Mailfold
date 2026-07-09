package webmail

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// sentSpecialUse is the IMAP special-use attribute marking the Sent folder.
const sentSpecialUse = "\\Sent"

// OutgoingMessage is a message composed by the user to be submitted for delivery.
type OutgoingMessage struct {
	To      []string `json:"to"`
	Cc      []string `json:"cc"`
	Bcc     []string `json:"bcc"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
	HTML    string   `json:"html"`
}

// validate rejects header/CRLF injection at the primitive so every caller — the
// webmail UI and API-key send endpoint alike — is protected. A raw CR or LF in a
// header value (a pure-ASCII Subject in particular, which go-message does not
// Q-encode) would otherwise be passed through verbatim and let a caller inject
// arbitrary headers or extra recipients.
func (m *OutgoingMessage) validate() error {
	if strings.ContainsAny(m.Subject, "\r\n") {
		return fmt.Errorf("subject must not contain a line break")
	}
	for _, list := range [][]string{m.To, m.Cc, m.Bcc} {
		for _, addr := range list {
			if strings.ContainsAny(addr, "\r\n") {
				return fmt.Errorf("recipient address must not contain a line break")
			}
		}
	}
	return nil
}

// ValidateOutgoing exposes validate to other packages (e.g. the scheduled-send
// create endpoint, which must reject a CRLF-injection attempt at creation
// time with 400 rather than only discovering it when the dispatcher later
// calls Send).
func ValidateOutgoing(m *OutgoingMessage) error { return m.validate() }

// recipients returns every envelope recipient (To + Cc + Bcc).
func (m *OutgoingMessage) recipients() []string {
	out := make([]string, 0, len(m.To)+len(m.Cc)+len(m.Bcc))
	out = append(out, m.To...)
	out = append(out, m.Cc...)
	out = append(out, m.Bcc...)
	return out
}

// normalizeRecipients punycode-encodes every recipient's domain in place. A
// punycode domain is always a valid, equivalent ASCII form of the same
// address, so this never changes where mail is delivered — it only avoids
// handing a receiving MTA (in particular mailcow's own postfix, which only
// recognizes its local mailboxes by their punycode form) a Unicode domain it
// may not resolve to the same mailbox as its ASCII form.
func (m *OutgoingMessage) normalizeRecipients() {
	for _, list := range [][]string{m.To, m.Cc, m.Bcc} {
		for i, addr := range list {
			list[i] = normalizeAddress(addr)
		}
	}
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
	if err := msg.validate(); err != nil {
		return err
	}
	// Normalized once and reused everywhere below (From header, SMTP AUTH,
	// MAIL FROM envelope) so all three agree — see normalizeAddress.
	email = normalizeAddress(email)
	msg.normalizeRecipients()
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

// SaveToSent appends a copy of a composed message to the user's Sent folder,
// marked \Seen. It is best-effort and meant to run after Send: the message has
// already been submitted over SMTP, so a failure here only means the sent copy is
// missing, not that delivery failed.
func (c *Client) SaveToSent(email, password string, msg *OutgoingMessage) error {
	// Normalized so the saved copy's From header matches what Send actually
	// used on the wire, not the raw address the caller happened to type.
	email = normalizeAddress(email)
	raw, err := renderMessage(email, msg)
	if err != nil {
		return err
	}
	ic, err := c.dial(email, password)
	if err != nil {
		return err
	}
	defer func() { _ = ic.Logout() }()

	sent := sentMailbox(ic)
	flags := []string{imap.SeenFlag}
	now := time.Now()
	if err := ic.Append(sent, flags, now, bytes.NewReader(raw)); err != nil {
		// The Sent folder may not exist yet; create it and retry once. A fresh
		// reader is needed because the first attempt consumed the previous one.
		_ = ic.Create(sent)
		return ic.Append(sent, flags, now, bytes.NewReader(raw))
	}
	return nil
}

// sentMailbox resolves the Sent folder name, preferring the mailbox flagged with
// the \Sent special-use attribute and falling back to the conventional "Sent".
func sentMailbox(ic *client.Client) string {
	ch := make(chan *imap.MailboxInfo, 32)
	done := make(chan error, 1)
	go func() { done <- ic.List("", "*", ch) }()
	name := "Sent"
	for m := range ch {
		for _, attr := range m.Attributes {
			if attr == sentSpecialUse {
				name = m.Name
			}
		}
	}
	<-done
	return name
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
