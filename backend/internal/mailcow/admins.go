package mailcow

import (
	"context"
	"encoding/json"
)

// Admins returns every super-administrator known to mailcow as raw JSON
// passthrough. The payload is forwarded untouched because the frontend renders
// the admin list directly, and re-modelling mailcow's partially stable shape
// here would add no value.
func (c *Client) Admins(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/admin/all")
}

// AddAdmin creates a super-administrator from the attributes in attr (username,
// password, and activation flags). It delegates to action so all mutating calls
// share one request/response implementation.
func (c *Client) AddAdmin(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/admin", attr)
}

// EditAdmin modifies one or more super-administrators, applying attr to each
// identifier in items. The identifiers and attributes are wrapped in the
// standard EditRequest shape that mailcow's edit endpoints require.
func (c *Client) EditAdmin(ctx context.Context, items []string, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/admin", EditRequest{Items: items, Attr: attr})
}

// DeleteAdmin removes one or more super-administrators identified by username.
// mailcow expects a bare array of identifiers for deletion, so items is
// forwarded unchanged.
func (c *Client) DeleteAdmin(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/admin", items)
}
