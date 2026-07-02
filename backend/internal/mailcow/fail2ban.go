package mailcow

import (
	"context"
	"encoding/json"
)

// Fail2Ban returns the raw Fail2Ban configuration and live state from mailcow,
// including tunables such as ban time and retry limits together with the
// currently banned and whitelisted networks.
//
// The payload is forwarded as json.RawMessage because it is only surfaced to
// the operator for inspection; keeping it untyped avoids coupling this client
// to the exact shape mailcow happens to emit. The GET is delegated to rawGet,
// which owns request construction and error handling in client.go.
func (c *Client) Fail2Ban(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/fail2ban")
}

// EditFail2Ban updates the Fail2Ban configuration with the supplied attributes
// (for example a new ban time, retry count, or whitelist/blacklist entry).
//
// mailcow exposes Fail2Ban as a single edit target, so the request always
// addresses the fixed item identifier "fail2ban" and carries the caller's
// changes in Attr. The call is routed through the shared action helper, which
// wraps the body and returns the standard []ActionResult response.
func (c *Client) EditFail2Ban(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/fail2ban", EditRequest{Items: []string{"fail2ban"}, Attr: attr})
}
