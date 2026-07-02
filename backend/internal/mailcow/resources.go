package mailcow

import (
	"context"
	"encoding/json"
)

// Resources returns every configured resource as raw JSON passthrough. A mailcow
// resource is a bookable shared object such as a meeting room or equipment mailbox.
// The payload is forwarded untouched because the frontend renders the resource list
// directly and its shape is only partially stable.
func (c *Client) Resources(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/resource/all")
}

// AddResource creates a new resource from the attributes in attr (display name,
// domain, kind, and so on). It delegates to action so all mutating calls share one
// request/response implementation.
func (c *Client) AddResource(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/resource", attr)
}

// EditResource modifies one or more resources, applying attr to each identifier in
// items. The identifiers and attributes are wrapped in the standard EditRequest
// shape that mailcow's edit endpoints require.
func (c *Client) EditResource(ctx context.Context, items []string, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/resource", EditRequest{Items: items, Attr: attr})
}

// DeleteResource removes one or more resources identified by name. mailcow expects
// a bare array of identifiers for deletion, so items is forwarded unchanged.
func (c *Client) DeleteResource(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/resource", items)
}
