package mailcow

import (
	"context"
	"encoding/json"
)

// DKIM returns the DKIM configuration for a single domain as raw JSON. The
// payload (public key, selector, key size) is passed straight through to the
// frontend, so it is returned untouched rather than decoded into a typed struct.
// The domain is interpolated into the path because mailcow scopes this endpoint
// per domain.
func (c *Client) DKIM(ctx context.Context, domain string) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/dkim/"+domain)
}

// AddDKIM generates a new DKIM key pair for the domains named in attr. A typical
// payload looks like {"domains":"a.com","dkim_selector":"dkim","key_size":"2048"}.
// It delegates to action so all mutating calls share one request/response path.
func (c *Client) AddDKIM(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/dkim", attr)
}

// DeleteDKIM removes the DKIM keys for the given domains. mailcow's delete
// endpoint expects a bare array of domain names, so items is forwarded unchanged.
func (c *Client) DeleteDKIM(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/dkim", items)
}
