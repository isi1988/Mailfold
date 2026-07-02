package mailcow

import (
	"context"
	"encoding/json"
)

// TLSPolicies returns every TLS policy map entry configured in mailcow as raw
// JSON.
//
// A TLS policy map pins the outbound SMTP encryption behaviour (for example
// "encrypt" or "dane") for a given destination. The consumer only lists and
// acts on these entries, so the payload is passed through as json.RawMessage
// instead of being decoded here. The GET is delegated to rawGet in client.go.
func (c *Client) TLSPolicies(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/tls-policy-map/all")
}

// AddTLSPolicy creates a new TLS policy map entry from the caller-supplied
// attributes.
//
// The attributes are sent to mailcow's add endpoint unchanged and the call is
// routed through the shared action helper, which returns the standard
// []ActionResult response.
func (c *Client) AddTLSPolicy(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/tls-policy-map", attr)
}

// DeleteTLSPolicy removes TLS policy map entries identified by their mailcow
// record ids.
//
// mailcow expects the ids as a bare JSON array, so items is sent as the request
// body unchanged. The delete is routed through the shared action helper, which
// returns the standard []ActionResult describing the outcome for each id.
func (c *Client) DeleteTLSPolicy(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/tls-policy-map", items)
}
