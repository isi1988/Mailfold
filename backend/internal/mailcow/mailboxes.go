package mailcow

import "context"

// Mailbox is a subset of the mailcow mailbox object returned by
// GET /api/v1/get/mailbox/all. It exposes only the fields the backend needs, so
// unknown fields in the upstream payload are safely ignored during decoding.
type Mailbox struct {
	// Username is the full mailbox address (for example "user@example.com") and
	// serves as the unique identifier when editing or deleting the mailbox.
	Username string `json:"username"`
	// Domain is the domain part of the mailbox address, provided by mailcow for
	// convenient grouping without having to parse Username.
	Domain string `json:"domain"`
	// Name is the display name (the person or role) associated with the mailbox.
	Name string `json:"name"`
	// Active indicates whether the mailbox can send/receive mail. mailcow encodes
	// this as an integer flag (1 = active, 0 = disabled).
	Active int `json:"active"`
	// QuotaKB is the mailbox storage quota expressed in kilobytes, matching the
	// unit mailcow reports. mailcow may quote this value as a string, so it uses
	// FlexInt64.
	QuotaKB FlexInt64 `json:"quota"`
	// MessagesTotal is the number of messages currently stored in the mailbox,
	// which mailcow may also return as a quoted string.
	MessagesTotal FlexInt64 `json:"messages"`
}

// Mailboxes returns every mailbox known to mailcow. It decodes into the typed
// Mailbox slice because the UI presents these fields directly, so a strongly
// typed result is preferred over raw passthrough.
func (c *Client) Mailboxes(ctx context.Context) ([]Mailbox, error) {
	return getList[Mailbox](ctx, c, "/api/v1/get/mailbox/all")
}

// AddMailbox creates a new mailbox from the attributes in attr (address, quota,
// password, and so on). It delegates to action so all mutating calls share one
// request/response implementation.
func (c *Client) AddMailbox(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/mailbox", attr)
}

// EditMailbox modifies one or more mailboxes, applying attr to each username in
// items. The identifiers and attributes are wrapped in the standard EditRequest
// shape mailcow expects.
func (c *Client) EditMailbox(ctx context.Context, items []string, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/mailbox", EditRequest{Items: items, Attr: attr})
}

// DeleteMailbox removes one or more mailboxes identified by username. mailcow
// expects a bare array of identifiers, so items is forwarded unchanged.
func (c *Client) DeleteMailbox(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/mailbox", items)
}
