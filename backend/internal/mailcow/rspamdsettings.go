package mailcow

import (
	"context"
	"encoding/json"
)

// RspamdSettings returns every configured Rspamd setting as raw JSON passthrough.
// An Rspamd setting is a named rule block that overrides the spam filter's
// behaviour for a matched set of messages (for example, whitelisting a sender or
// forcing a symbol score), so mailcow exposes them as a flat list. The payload is
// returned untouched because the frontend renders it directly.
func (c *Client) RspamdSettings(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/rsetting/all")
}

// AddRspamdSetting creates a new Rspamd setting from the attributes in attr (the
// descriptive name, the raw rule content, and whether it is active). It delegates
// to action so all mutating calls share one request/response implementation.
func (c *Client) AddRspamdSetting(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/rsetting", attr)
}

// DeleteRspamdSetting removes one or more Rspamd settings identified by id.
// mailcow expects a bare array of identifiers for deletion, so items is forwarded
// unchanged.
func (c *Client) DeleteRspamdSetting(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/rsetting", items)
}
