package webmail

import (
	"fmt"

	"github.com/emersion/go-imap"
)

// flagNames maps friendly flag names (as used by the API) to IMAP system flags.
var flagNames = map[string]string{
	"seen":     imap.SeenFlag,
	"flagged":  imap.FlaggedFlag,
	"answered": imap.AnsweredFlag,
	"deleted":  imap.DeletedFlag,
	"draft":    imap.DraftFlag,
}

// SetFlag adds or removes a system flag (seen, flagged, answered, deleted,
// draft) on a message identified by UID.
func (c *Client) SetFlag(email, password, folder string, uid uint32, flag string, set bool) error {
	imapFlag, ok := flagNames[flag]
	if !ok {
		return fmt.Errorf("unknown flag %q", flag)
	}
	ic, err := c.dial(email, password)
	if err != nil {
		return err
	}
	defer func() { _ = ic.Logout() }()

	if _, err := ic.Select(folder, false); err != nil {
		return err
	}
	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)
	op := imap.FlagsOp(imap.AddFlags)
	if !set {
		op = imap.RemoveFlags
	}
	return ic.UidStore(seqset, imap.FormatFlagsOp(op, true), []interface{}{imapFlag}, nil)
}

// Move copies a message to the target folder and removes it from the source,
// implemented with COPY + \Deleted + EXPUNGE for broad server compatibility.
func (c *Client) Move(email, password, folder string, uid uint32, target string) error {
	ic, err := c.dial(email, password)
	if err != nil {
		return err
	}
	defer func() { _ = ic.Logout() }()

	if _, err := ic.Select(folder, false); err != nil {
		return err
	}
	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)
	if err := ic.UidCopy(seqset, target); err != nil {
		return err
	}
	if err := ic.UidStore(seqset, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil); err != nil {
		return err
	}
	return ic.Expunge(nil)
}

// Delete permanently removes a message: it marks it \Deleted and expunges.
func (c *Client) Delete(email, password, folder string, uid uint32) error {
	ic, err := c.dial(email, password)
	if err != nil {
		return err
	}
	defer func() { _ = ic.Logout() }()

	if _, err := ic.Select(folder, false); err != nil {
		return err
	}
	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)
	if err := ic.UidStore(seqset, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil); err != nil {
		return err
	}
	return ic.Expunge(nil)
}

// CreateFolder creates a new mailbox (folder) for the user.
func (c *Client) CreateFolder(email, password, name string) error {
	ic, err := c.dial(email, password)
	if err != nil {
		return err
	}
	defer func() { _ = ic.Logout() }()
	return ic.Create(name)
}
