package mailcow

import "context"

// Domain is a subset of a mailcow mail-domain object as returned by
// GET /api/v1/get/domain/all. It exists to give the backend a stable, typed view
// over just the fields the UI needs; any additional fields mailcow returns are
// intentionally ignored so upstream schema additions do not break decoding.
type Domain struct {
	// DomainName is the fully-qualified domain (for example "example.com") and
	// is the primary identifier used when editing or deleting the domain.
	DomainName string `json:"domain_name"`
	// Description is the free-text label an administrator attached to the domain.
	Description string `json:"description"`
	// Active indicates whether mail for the domain is enabled. mailcow encodes
	// this as an integer flag (1 = active, 0 = disabled) rather than a boolean.
	Active int `json:"active"`
	// Mailboxes is the current number of mailboxes provisioned in the domain.
	Mailboxes int `json:"mboxes_in_domain"`
	// Aliases is the current number of aliases defined in the domain.
	Aliases int `json:"aliases_in_domain"`
	// MaxQuota is the maximum total storage (in bytes) allowed for the domain.
	MaxQuota int64 `json:"max_quota_for_domain"`
	// QuotaUsed is the storage (in bytes) currently consumed across the domain.
	QuotaUsed int64 `json:"bytes_total"`
}

// Domains returns every mail domain known to mailcow. It decodes into the typed
// Domain slice because the frontend renders these fields directly, so a strongly
// typed result is preferable to raw passthrough here.
func (c *Client) Domains(ctx context.Context) ([]Domain, error) {
	var out []Domain
	if err := c.get(ctx, "/api/v1/get/domain/all", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddDomain creates a mail domain. attr is the mailcow add payload describing the
// new domain's attributes. It delegates to action so the add/edit/delete request
// plumbing lives in a single place.
func (c *Client) AddDomain(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/domain", attr)
}

// EditDomain modifies one or more domains, applying attr to each name in items.
// It wraps the identifiers and attributes in the standard EditRequest shape that
// mailcow's edit endpoints require.
func (c *Client) EditDomain(ctx context.Context, items []string, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/domain", EditRequest{Items: items, Attr: attr})
}

// DeleteDomain removes one or more domains identified by name. mailcow expects a
// bare array of identifiers for deletion, so items is passed through unchanged.
func (c *Client) DeleteDomain(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/domain", items)
}
