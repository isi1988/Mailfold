package mailcow

import (
	"context"
	"encoding/json"
)

// Relayhosts returns every configured relayhost as raw JSON passthrough. A
// relayhost is an upstream smarthost that mailcow forwards outbound mail
// through, complete with authentication credentials. The payload is returned
// untouched because the frontend renders the relayhost list directly.
func (c *Client) Relayhosts(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/relayhost/all")
}

// AddRelayhost creates a new relayhost from the attributes in attr (hostname,
// port, and optional credentials). It delegates to action so all mutating calls
// share one request/response implementation.
func (c *Client) AddRelayhost(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/relayhost", attr)
}

// EditRelayhost modifies one or more relayhosts, applying attr to each
// identifier in items. The identifiers and attributes are wrapped in the
// standard EditRequest shape that mailcow's edit endpoints require.
func (c *Client) EditRelayhost(ctx context.Context, items []string, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/relayhost", EditRequest{Items: items, Attr: attr})
}

// DeleteRelayhost removes one or more relayhosts identified by id. mailcow
// expects a bare array of identifiers for deletion, so items is forwarded
// unchanged.
func (c *Client) DeleteRelayhost(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/relayhost", items)
}
