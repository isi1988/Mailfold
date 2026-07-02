package mailcow

import (
	"context"
	"encoding/json"
)

// PolicyAllow returns the per-domain allow list (whitelist) policy entries for
// the given domain.
//
// Senders matched by an allow entry bypass mailcow's spam and rejection checks.
// The domain is appended to the mailcow path as a path segment, and the raw
// response is forwarded as json.RawMessage because the caller only renders the
// entries. The GET is delegated to rawGet in client.go.
func (c *Client) PolicyAllow(ctx context.Context, domain string) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/policy_wl_domain/"+domain)
}

// PolicyDeny returns the per-domain deny list (blacklist) policy entries for the
// given domain.
//
// Senders matched by a deny entry are rejected for that domain. As with the
// allow list, the domain is supplied as a path segment and the raw response is
// passed through as json.RawMessage. The GET is delegated to rawGet in
// client.go.
func (c *Client) PolicyDeny(ctx context.Context, domain string) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/policy_bl_domain/"+domain)
}

// AddPolicy creates a new domain policy entry (either an allow or a deny rule,
// as encoded in attr) from the caller-supplied attributes.
//
// The attributes are sent to mailcow's add endpoint unchanged and the call is
// routed through the shared action helper, which returns the standard
// []ActionResult response.
func (c *Client) AddPolicy(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/domain-policy", attr)
}

// DeletePolicy removes domain policy entries identified by their mailcow policy
// record ids (prefids).
//
// mailcow expects the ids as a bare JSON array, so items is passed through as
// the request body. The delete is routed through the shared action helper,
// which returns the standard []ActionResult response.
func (c *Client) DeletePolicy(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/domain-policy", items)
}
