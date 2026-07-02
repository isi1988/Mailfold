package mailcow

import (
	"context"
	"encoding/json"
)

// OAuth2Clients returns every OAuth2 client registered in mailcow as raw JSON.
//
// OAuth2 client records carry credential and redirect-URI details whose exact
// shape varies with the mailcow version, and the consumer only lists them, so
// the payload is passed through as json.RawMessage rather than decoded here. The
// GET is delegated to rawGet in client.go.
func (c *Client) OAuth2Clients(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/oauth2-client/all")
}

// AddOAuth2Client registers a new OAuth2 client from the attributes in attr
// (such as its redirect URI). It delegates to the shared action helper so all
// mutating calls share one request/response implementation, returning the
// standard []ActionResult describing the outcome.
func (c *Client) AddOAuth2Client(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/oauth2-client", attr)
}

// DeleteOAuth2Client removes one or more OAuth2 clients identified by their
// mailcow ids. The items are forwarded unchanged because mailcow's delete
// endpoint expects a bare JSON array of identifiers, and the call is routed
// through the shared action helper.
func (c *Client) DeleteOAuth2Client(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/oauth2-client", items)
}
