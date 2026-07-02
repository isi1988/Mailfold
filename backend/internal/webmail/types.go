// Package webmail is an IMAP/SMTP-backed webmail layer. Unlike the mailcow
// admin API (which configures the mail server), this package acts on behalf of
// an end user: it authenticates with the user's own mailbox credentials and
// reads/sends their mail over IMAP and SMTP.
package webmail

import "time"

// Address is a mail address with an optional display name.
type Address struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Folder is an IMAP mailbox (folder) the user can browse.
type Folder struct {
	Name       string   `json:"name"`
	Attributes []string `json:"attributes"`
}

// MessageHeader is the summary of a message shown in a folder listing.
type MessageHeader struct {
	UID     uint32    `json:"uid"`
	Subject string    `json:"subject"`
	From    []Address `json:"from"`
	To      []Address `json:"to"`
	Date    time.Time `json:"date"`
	Flags   []string  `json:"flags"`
	Seen    bool      `json:"seen"`
	Size    uint32    `json:"size"`
}

// Attachment describes a non-inline message part.
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
}

// Message is a fully read message: its header plus decoded body and attachment
// metadata.
type Message struct {
	MessageHeader
	Text        string       `json:"text"`
	HTML        string       `json:"html"`
	Attachments []Attachment `json:"attachments"`
}
