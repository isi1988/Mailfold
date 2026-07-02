package mailcow

import (
	"context"
	"encoding/json"
)

// Filters returns all Sieve filters as raw JSON passthrough.
func (c *Client) Filters(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/filters/all")
}

// AddFilter creates a Sieve filter from the given attributes (mailbox, type,
// script, and so on).
func (c *Client) AddFilter(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/filter", attr)
}

// EditFilter updates one or more filters identified by their ids.
func (c *Client) EditFilter(ctx context.Context, items []string, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/filter", EditRequest{Items: items, Attr: attr})
}

// DeleteFilter removes one or more filters by id.
func (c *Client) DeleteFilter(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/filter", items)
}
