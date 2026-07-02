package mailcow

import (
	"context"
	"encoding/json"
)

// DomainAdmins returns every domain administrator known to mailcow as raw JSON
// passthrough. The payload is forwarded untouched because the frontend renders
// the domain-admin list directly, and re-modelling mailcow's partially stable
// shape here would add no value.
func (c *Client) DomainAdmins(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/domain-admin/all")
}

// AddDomainAdmin creates a domain administrator from the attributes in attr
// (username, password, and the domains the admin may manage). It delegates to
// action so all mutating calls share one request/response implementation.
func (c *Client) AddDomainAdmin(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/domain-admin", attr)
}

// EditDomainAdmin modifies one or more domain administrators, applying attr to
// each identifier in items. The identifiers and attributes are wrapped in the
// standard EditRequest shape that mailcow's edit endpoints require.
func (c *Client) EditDomainAdmin(ctx context.Context, items []string, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/domain-admin", EditRequest{Items: items, Attr: attr})
}

// DeleteDomainAdmin removes one or more domain administrators identified by
// username. mailcow expects a bare array of identifiers for deletion, so items
// is forwarded unchanged.
func (c *Client) DeleteDomainAdmin(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/domain-admin", items)
}
