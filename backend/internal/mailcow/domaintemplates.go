package mailcow

import (
	"context"
	"encoding/json"
)

// DomainTemplates returns every configured domain template as raw JSON
// passthrough. A domain template captures the default limits and feature flags
// (quota, mailbox count, aliases, and so on) applied when a new domain is
// created from it, so its shape is broad and only partially stable. The payload
// is returned untouched because the frontend renders the template list directly.
func (c *Client) DomainTemplates(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/domain/template/all")
}

// AddDomainTemplate creates a new domain template from the attributes in attr
// (template name plus the domain defaults it carries). It delegates to action so
// all mutating calls share one request/response implementation.
func (c *Client) AddDomainTemplate(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/domain/template", attr)
}

// EditDomainTemplate modifies one or more domain templates, applying attr to
// each identifier in items. The identifiers and attributes are wrapped in the
// standard EditRequest shape that mailcow's edit endpoints require.
func (c *Client) EditDomainTemplate(ctx context.Context, items []string, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/domain/template", EditRequest{Items: items, Attr: attr})
}

// DeleteDomainTemplate removes one or more domain templates identified by items.
// mailcow expects a bare array of identifiers for deletion, so items is
// forwarded unchanged.
func (c *Client) DeleteDomainTemplate(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/domain/template", items)
}
