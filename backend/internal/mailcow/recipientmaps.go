package mailcow

import (
	"context"
	"encoding/json"
)

// RecipientMaps returns every configured recipient map as raw JSON passthrough.
// A recipient map rewrites the envelope recipient of inbound mail, redirecting
// messages addressed to one address towards another before delivery. mailcow
// exposes them as a flat list whose shape the backend does not model, so the
// payload is returned untouched for the frontend to render directly.
func (c *Client) RecipientMaps(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/recipient_map/all")
}

// AddRecipientMap creates a new recipient map from the attributes in attr
// (typically the original recipient and the address it should be rewritten to).
// It delegates to action so all mutating calls share one request/response
// implementation.
func (c *Client) AddRecipientMap(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/recipient_map", attr)
}

// DeleteRecipientMap removes one or more recipient maps identified by id.
// mailcow expects a bare array of identifiers for deletion, so items is
// forwarded unchanged.
func (c *Client) DeleteRecipientMap(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/recipient_map", items)
}
