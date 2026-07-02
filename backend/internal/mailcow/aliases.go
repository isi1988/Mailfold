package mailcow

import "context"

// Alias represents a single mailcow alias entry as returned by
// GET /api/v1/get/alias/all. An alias forwards mail arriving at one address to
// one or more destination addresses. Only the fields the backend uses are
// modelled; anything else in the payload is ignored.
type Alias struct {
	// ID is mailcow's numeric identifier for the alias. Unlike domains and
	// mailboxes, aliases are edited and deleted by this id (passed as a string),
	// because their source address is not guaranteed to be unique-friendly.
	ID int `json:"id"`
	// Address is the source address that receives mail (the "from" side of the
	// forward). It may be a full address or a catch-all pattern.
	Address string `json:"address"`
	// Goto is the destination the alias forwards to. mailcow stores multiple
	// destinations as a comma-separated string in this single field.
	Goto string `json:"goto"`
	// Domain is the domain the source address belongs to, supplied by mailcow so
	// callers can group aliases without parsing Address.
	Domain string `json:"domain"`
	// Active indicates whether the alias is in effect. mailcow encodes this as an
	// integer flag (1 = active, 0 = disabled).
	Active int `json:"active"`
}

// Aliases returns every alias known to mailcow. It decodes into the typed Alias
// slice because the UI displays these fields directly, so a typed result is
// preferable to raw passthrough.
func (c *Client) Aliases(ctx context.Context) ([]Alias, error) {
	return getList[Alias](ctx, c, "/api/v1/get/alias/all")
}

// AddAlias creates a new alias from the attributes in attr (source address and
// forwarding targets). It delegates to action so all mutating calls share one
// request/response implementation.
func (c *Client) AddAlias(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/alias", attr)
}

// EditAlias updates one or more aliases, applying attr to each id in items.
// Because aliases are keyed by numeric id, the items are the alias ids rendered
// as strings, wrapped in the standard EditRequest shape.
func (c *Client) EditAlias(ctx context.Context, items []string, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/alias", EditRequest{Items: items, Attr: attr})
}

// DeleteAlias removes one or more aliases identified by id. The items are alias
// ids rendered as strings, forwarded unchanged because mailcow's delete endpoint
// expects a bare array of identifiers.
func (c *Client) DeleteAlias(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/alias", items)
}
