package mailcow

import (
	"context"
	"encoding/json"
)

// BCCMaps returns every configured BCC map as raw JSON passthrough. A BCC map
// silently forwards a copy of a mailbox or domain's mail to another address
// (used for archiving or lawful interception), so mailcow exposes them as a flat
// list. The payload is returned untouched because the frontend renders it
// directly.
func (c *Client) BCCMaps(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/bcc/all")
}

// AddBCC creates a new BCC map from the attributes in attr (the local
// destination whose mail is copied, the recipient address, and the direction).
// It delegates to action so all mutating calls share one request/response
// implementation.
func (c *Client) AddBCC(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/bcc", attr)
}

// DeleteBCC removes one or more BCC maps identified by id. mailcow expects a
// bare array of identifiers for deletion, so items is forwarded unchanged.
func (c *Client) DeleteBCC(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/bcc", items)
}
