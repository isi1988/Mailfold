package mailcow

import (
	"context"
	"encoding/json"
)

// RateLimitDomains returns the per-domain rate limits configured in mailcow as
// raw JSON passthrough.
//
// A domain rate limit caps the volume of outbound mail a whole domain may send
// within a time window. mailcow exposes these limits as a flat list whose shape
// the backend does not model, so the payload is returned untouched via rawGet
// for the frontend to render directly.
func (c *Client) RateLimitDomains(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/rl-domain/all")
}

// EditRateLimitDomain updates the rate limit for one or more domains, applying
// attr to each identifier in items. The identifiers and attributes are wrapped
// in the standard EditRequest shape that mailcow's edit endpoints require, and
// the call is routed through the shared action helper.
func (c *Client) EditRateLimitDomain(ctx context.Context, items []string, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/rl-domain", EditRequest{Items: items, Attr: attr})
}
