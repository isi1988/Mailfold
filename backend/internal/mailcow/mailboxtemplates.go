package mailcow

import (
	"context"
	"encoding/json"
)

// MailboxTemplates returns every mailbox template known to mailcow as raw JSON
// passthrough. A mailbox template bundles default settings (quota, allowed
// protocols, spam and Sieve options, and similar) that mailcow applies when a
// new mailbox is created from it, so administrators do not have to configure
// each mailbox by hand. The payload is returned untouched because the frontend
// renders it directly.
func (c *Client) MailboxTemplates(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/mailbox/template/all")
}

// AddMailboxTemplate creates a new mailbox template from the attributes in attr
// (the template name and the default mailbox settings it carries). It delegates
// to action so all mutating calls share one request/response implementation.
func (c *Client) AddMailboxTemplate(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/mailbox/template", attr)
}

// EditMailboxTemplate updates one or more mailbox templates, applying attr to
// each identifier in items. The items are the template ids rendered as strings,
// wrapped in the standard EditRequest shape mailcow expects for edits.
func (c *Client) EditMailboxTemplate(ctx context.Context, items []string, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/mailbox/template", EditRequest{Items: items, Attr: attr})
}

// DeleteMailboxTemplate removes one or more mailbox templates identified by id.
// mailcow expects a bare array of identifiers for deletion, so items is
// forwarded unchanged.
func (c *Client) DeleteMailboxTemplate(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/mailbox/template", items)
}
