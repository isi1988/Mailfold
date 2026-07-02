package mailcow

import (
	"context"
	"encoding/json"
)

// Containers returns the status map of every mailcow Docker container (image,
// state and started-at information) as raw JSON.
//
// The payload is forwarded as json.RawMessage because it is used purely for
// display on the status dashboard and its shape tracks the underlying mailcow
// and Docker versions. The GET is delegated to rawGet in client.go.
func (c *Client) Containers(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/status/containers")
}

// Version returns the installed mailcow version as raw JSON.
//
// The value is passed through as json.RawMessage so callers can surface it
// verbatim without this client needing to track the exact response shape. The
// GET is delegated to rawGet in client.go.
func (c *Client) Version(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/status/version")
}

// Vmail returns mailcow's mail storage (vmail volume) usage as raw JSON,
// reporting how much disk space the mailboxes consume.
//
// The payload is forwarded as json.RawMessage because it is only displayed to
// the operator. The GET is delegated to rawGet in client.go.
func (c *Client) Vmail(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/status/vmail")
}
