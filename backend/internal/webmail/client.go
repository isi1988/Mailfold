package webmail

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sort"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

// maxMessages caps how many message headers a single listing returns.
const maxMessages = 200

// Client performs webmail operations against an IMAP server (and, for sending,
// an SMTP server) using per-user credentials supplied on each call.
type Client struct {
	imapAddr    string
	smtpAddr    string
	useTLS      bool
	insecureTLS bool
}

// NewClient creates a webmail client. useTLS selects implicit TLS (IMAPS/SMTPS)
// versus a plaintext dial; insecureTLS skips certificate verification (for
// self-signed development certificates).
func NewClient(imapAddr, smtpAddr string, useTLS, insecureTLS bool) *Client {
	return &Client{imapAddr: imapAddr, smtpAddr: smtpAddr, useTLS: useTLS, insecureTLS: insecureTLS}
}

// Configured reports whether an IMAP address was provided; when false, webmail
// is effectively disabled.
func (c *Client) Configured() bool { return c.imapAddr != "" }

// dial opens an authenticated IMAP connection for the given user.
func (c *Client) dial(email, password string) (*client.Client, error) {
	var (
		ic  *client.Client
		err error
	)
	if c.useTLS {
		host, _, _ := net.SplitHostPort(c.imapAddr)
		ic, err = client.DialTLS(c.imapAddr, &tls.Config{ServerName: host, InsecureSkipVerify: c.insecureTLS}) //nolint:gosec // insecure is an opt-in dev flag
	} else {
		ic, err = client.Dial(c.imapAddr)
	}
	if err != nil {
		return nil, err
	}
	if err := ic.Login(email, password); err != nil {
		_ = ic.Logout()
		return nil, err
	}
	return ic, nil
}

// Verify checks that the credentials can authenticate to IMAP.
func (c *Client) Verify(email, password string) error {
	ic, err := c.dial(email, password)
	if err != nil {
		return err
	}
	return ic.Logout()
}

// Folders lists the user's mailboxes.
func (c *Client) Folders(email, password string) ([]Folder, error) {
	ic, err := c.dial(email, password)
	if err != nil {
		return nil, err
	}
	defer func() { _ = ic.Logout() }()

	ch := make(chan *imap.MailboxInfo, 20)
	done := make(chan error, 1)
	go func() { done <- ic.List("", "*", ch) }()

	folders := []Folder{}
	for m := range ch {
		folders = append(folders, Folder{Name: m.Name, Attributes: m.Attributes})
	}
	if err := <-done; err != nil {
		return nil, err
	}
	sort.Slice(folders, func(i, j int) bool { return folders[i].Name < folders[j].Name })
	return folders, nil
}

// Messages lists up to limit of the most recent message headers in a folder.
func (c *Client) Messages(email, password, folder string, limit int) ([]MessageHeader, error) {
	ic, err := c.dial(email, password)
	if err != nil {
		return nil, err
	}
	defer func() { _ = ic.Logout() }()

	mbox, err := ic.Select(folder, true)
	if err != nil {
		return nil, err
	}
	if mbox.Messages == 0 {
		return []MessageHeader{}, nil
	}
	if limit <= 0 || limit > maxMessages {
		limit = 50
	}
	from := uint32(1)
	if mbox.Messages > uint32(limit) {
		from = mbox.Messages - uint32(limit) + 1
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, mbox.Messages)

	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid, imap.FetchInternalDate, imap.FetchRFC822Size}
	ch := make(chan *imap.Message, limit)
	done := make(chan error, 1)
	go func() { done <- ic.Fetch(seqset, items, ch) }()

	out := []MessageHeader{}
	for m := range ch {
		out = append(out, headerFrom(m))
	}
	if err := <-done; err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UID > out[j].UID })
	return out, nil
}

// Message reads a single message by UID, decoding its text/HTML body and
// listing its attachments.
func (c *Client) Message(email, password, folder string, uid uint32) (*Message, error) {
	ic, err := c.dial(email, password)
	if err != nil {
		return nil, err
	}
	defer func() { _ = ic.Logout() }()

	if _, err := ic.Select(folder, false); err != nil {
		return nil, err
	}
	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem(), imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid, imap.FetchInternalDate, imap.FetchRFC822Size}
	ch := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() { done <- ic.UidFetch(seqset, items, ch) }()

	m := <-ch
	if err := <-done; err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("message uid %d not found", uid)
	}
	out := &Message{MessageHeader: headerFrom(m), Attachments: []Attachment{}}
	if body := m.GetBody(section); body != nil {
		_ = parseBody(body, out)
	}
	return out, nil
}

// headerFrom converts an IMAP message into our MessageHeader.
func headerFrom(m *imap.Message) MessageHeader {
	h := MessageHeader{UID: m.Uid, Flags: m.Flags, Size: m.Size, Date: m.InternalDate}
	for _, f := range m.Flags {
		if f == imap.SeenFlag {
			h.Seen = true
		}
	}
	if m.Envelope != nil {
		h.Subject = m.Envelope.Subject
		if !m.Envelope.Date.IsZero() {
			h.Date = m.Envelope.Date
		}
		h.From = addresses(m.Envelope.From)
		h.To = addresses(m.Envelope.To)
	}
	return h
}

func addresses(list []*imap.Address) []Address {
	out := []Address{}
	for _, a := range list {
		if a == nil {
			continue
		}
		out = append(out, Address{Name: a.PersonalName, Email: a.MailboxName + "@" + a.HostName})
	}
	return out
}

// parseBody walks the MIME structure, extracting plain-text and HTML bodies and
// recording attachment metadata.
func parseBody(r io.Reader, out *Message) error {
	mr, err := mail.CreateReader(r)
	if err != nil {
		return err
	}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch h := part.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, _ := h.ContentType()
			b, _ := io.ReadAll(part.Body)
			if contentType == "text/html" {
				out.HTML = string(b)
			} else {
				out.Text = string(b)
			}
		case *mail.AttachmentHeader:
			filename, _ := h.Filename()
			contentType, _, _ := h.ContentType()
			b, _ := io.ReadAll(part.Body)
			out.Attachments = append(out.Attachments, Attachment{Filename: filename, ContentType: contentType, Size: len(b)})
		}
	}
	return nil
}
