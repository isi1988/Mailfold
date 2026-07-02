package mailcow

import (
	"context"
	"encoding/json"
)

// Quarantine returns every message currently held in mailcow's quarantine as
// raw JSON.
//
// Quarantined items carry rich, version-dependent metadata (spam scores,
// recipients, subjects and identifiers) that the consumer only lists and acts
// on, so the payload is passed through as json.RawMessage instead of being
// decoded here. The GET is delegated to rawGet in client.go.
func (c *Client) Quarantine(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/quarantine/all")
}

// DeleteQuarantine permanently removes quarantined messages identified by their
// mailcow quarantine ids.
//
// mailcow expects the ids as a bare JSON array, so items is sent as the request
// body unchanged. The delete is routed through the shared action helper, which
// returns the standard []ActionResult describing the outcome for each id.
func (c *Client) DeleteQuarantine(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/qitem", items)
}
