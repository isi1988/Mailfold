package mailcow

import (
	"context"
	"encoding/json"
)

// ForwardingHosts returns every configured forwarding host as raw JSON
// passthrough. Forwarding hosts are trusted upstream relays whose mail bypasses
// spam and greylist checks, so mailcow exposes them as a flat list. The payload
// is returned untouched because the frontend renders it directly.
func (c *Client) ForwardingHosts(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/fwdhost/all")
}

// AddForwardingHost registers a new forwarding host from the attributes in attr
// (typically the host or network and whether its mail should skip spam checks).
// It delegates to action so all mutating calls share one request/response
// implementation.
func (c *Client) AddForwardingHost(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/fwdhost", attr)
}

// DeleteForwardingHost removes one or more forwarding hosts identified by host.
// mailcow expects a bare array of identifiers for deletion, so items is
// forwarded unchanged.
func (c *Client) DeleteForwardingHost(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/fwdhost", items)
}
