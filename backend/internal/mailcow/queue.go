package mailcow

import (
	"context"
	"encoding/json"
)

// MailQueue returns the raw Postfix mail queue exactly as mailcow reports it.
//
// The response is deliberately passed through as json.RawMessage rather than
// being decoded into a typed struct: the mailcow queue payload is large, its
// shape varies between mailcow versions, and the frontend only needs to render
// it, so decoding here would add maintenance cost without any benefit. The GET
// is delegated to rawGet, which centralizes request building, authentication,
// and error handling in client.go.
func (c *Client) MailQueue(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/mailq/all")
}

// FlushQueue asks mailcow to flush the Postfix mail queue, forcing an immediate
// delivery retry for every deferred message instead of waiting for the next
// scheduled queue run.
//
// mailcow models this as an "edit" action on the mailq resource whose only
// attribute is the requested action ("flush"). The call is routed through the
// shared action helper so it returns the standard []ActionResult that every
// mutating mailcow endpoint produces.
func (c *Client) FlushQueue(ctx context.Context) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/mailq", map[string]string{"action": "flush"})
}

// DeleteAllQueue permanently removes every message currently held in the
// Postfix queue. mailcow models this as the "super_delete" edit action —
// unlike "flush" (retry delivery), this discards the messages outright, so
// callers should confirm with the operator before calling it.
func (c *Client) DeleteAllQueue(ctx context.Context) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/mailq", map[string]string{"action": "super_delete"})
}
