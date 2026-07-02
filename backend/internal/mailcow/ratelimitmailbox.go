package mailcow

import (
	"context"
	"encoding/json"
)

// RateLimitMailboxes returns the per-mailbox rate limits configured in mailcow
// as raw JSON.
//
// A mailbox rate limit caps how many messages a single mailbox may send within
// a rolling window, which mailcow uses to contain compromised or misbehaving
// accounts. The consumer only lists and edits these entries, so the payload is
// passed through as json.RawMessage instead of being decoded here. The GET is
// delegated to rawGet in client.go.
func (c *Client) RateLimitMailboxes(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/rl-mbox/all")
}

// EditRateLimitMailbox updates the rate limit for one or more mailboxes,
// applying attr to each identifier in items.
//
// The identifiers and attributes are wrapped in the standard EditRequest shape
// mailcow expects and routed through the shared action helper, which returns
// the standard []ActionResult response.
func (c *Client) EditRateLimitMailbox(ctx context.Context, items []string, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/rl-mbox", EditRequest{Items: items, Attr: attr})
}
