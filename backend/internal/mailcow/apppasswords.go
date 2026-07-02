package mailcow

import (
	"context"
	"encoding/json"
)

// AppPasswords returns every application password configured for the given
// mailbox as raw JSON.
//
// mailcow scopes app passwords per mailbox, so the mailbox address is appended
// to the collection path. Each entry carries mostly opaque, version-dependent
// metadata (name, protocol scopes and identifiers) that the frontend only lists
// and acts on, so the payload is passed through as json.RawMessage rather than
// decoded here. The GET is delegated to rawGet in client.go.
func (c *Client) AppPasswords(ctx context.Context, mailbox string) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/app-passwd/all/"+mailbox)
}

// AddAppPassword creates a new application password from the attributes in attr
// (owning mailbox, display name, the password itself and its allowed
// protocols). It delegates to action so all mutating calls share one
// request/response implementation.
func (c *Client) AddAppPassword(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/app-passwd", attr)
}

// DeleteAppPassword removes one or more application passwords identified by id.
// mailcow expects a bare array of identifiers for deletion, so items is
// forwarded unchanged through the shared action helper.
func (c *Client) DeleteAppPassword(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/app-passwd", items)
}
