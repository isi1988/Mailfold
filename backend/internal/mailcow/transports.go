package mailcow

import (
	"context"
	"encoding/json"
)

// Transports returns every configured transport map as raw JSON passthrough. A
// transport map tells Postfix how to relay mail for a given destination (for
// example through a smarthost with explicit credentials), so its shape is large
// and only partially stable. The payload is returned untouched because the
// frontend renders the transport list directly.
func (c *Client) Transports(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/transport/all")
}

// AddTransport creates a new transport map from the attributes in attr
// (destination, next-hop, credentials, and so on). It delegates to action so all
// mutating calls share one request/response implementation.
func (c *Client) AddTransport(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/transport", attr)
}

// EditTransport modifies one or more transport maps, applying attr to each
// identifier in items. The identifiers and attributes are wrapped in the
// standard EditRequest shape that mailcow's edit endpoints require.
func (c *Client) EditTransport(ctx context.Context, items []string, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/transport", EditRequest{Items: items, Attr: attr})
}

// DeleteTransport removes one or more transport maps identified by id. mailcow
// expects a bare array of identifiers for deletion, so items is forwarded
// unchanged.
func (c *Client) DeleteTransport(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/transport", items)
}
