package mailcow

import (
	"context"
	"encoding/json"
)

// TempAliases returns the time-limited ("spam"/throwaway) aliases for a mailbox
// as raw JSON passthrough.
func (c *Client) TempAliases(ctx context.Context, mailbox string) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/time_limited_aliases/"+mailbox)
}

// AddTempAlias creates a time-limited alias from the given attributes.
func (c *Client) AddTempAlias(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/time_limited_alias", attr)
}
