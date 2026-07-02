package mailcow

import "context"

// EditPushover updates the Pushover push-notification settings for the mailboxes
// named in items using the attributes in attr.
//
// mailcow exposes no read endpoint for Pushover settings (they are part of the
// mailbox object), so only the edit operation is provided. The call is wrapped
// in the shared EditRequest and routed through the action helper, which returns
// the standard []ActionResult response.
func (c *Client) EditPushover(ctx context.Context, items []string, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/pushover", EditRequest{Items: items, Attr: attr})
}
