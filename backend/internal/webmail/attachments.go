package webmail

import (
	"fmt"
	"io"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-message/mail"
)

// Attachment fetches the raw bytes of the index-th attachment (0-based) of a
// message, returning its filename, content type, and data.
func (c *Client) Attachment(email, password, folder string, uid uint32, index int) (string, string, []byte, error) {
	ic, err := c.dial(email, password)
	if err != nil {
		return "", "", nil, err
	}
	defer func() { _ = ic.Logout() }()

	if _, err := ic.Select(folder, false); err != nil {
		return "", "", nil, err
	}
	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)
	section := &imap.BodySectionName{}
	ch := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() { done <- ic.UidFetch(seqset, []imap.FetchItem{section.FetchItem()}, ch) }()

	m := <-ch
	if err := <-done; err != nil {
		return "", "", nil, err
	}
	if m == nil {
		return "", "", nil, fmt.Errorf("message uid %d not found", uid)
	}
	body := m.GetBody(section)
	if body == nil {
		return "", "", nil, fmt.Errorf("message uid %d has no body", uid)
	}
	return nthAttachment(body, index)
}

// nthAttachment walks the MIME structure and returns the index-th attachment.
func nthAttachment(r io.Reader, index int) (string, string, []byte, error) {
	mr, err := mail.CreateReader(r)
	if err != nil {
		return "", "", nil, err
	}
	current := 0
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", nil, err
		}
		h, ok := part.Header.(*mail.AttachmentHeader)
		if !ok {
			continue
		}
		if current == index {
			filename, _ := h.Filename()
			contentType, _, _ := h.ContentType()
			data, err := io.ReadAll(part.Body)
			if err != nil {
				return "", "", nil, err
			}
			return filename, contentType, data, nil
		}
		current++
	}
	return "", "", nil, fmt.Errorf("attachment %d not found", index)
}
