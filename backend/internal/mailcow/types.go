package mailcow

// Domain is a subset of a mailcow mail-domain object as returned by
// GET /api/v1/get/domain/all. Unknown fields are ignored.
type Domain struct {
	DomainName  string `json:"domain_name"`
	Description string `json:"description"`
	Active      int    `json:"active"`
	Mailboxes   int    `json:"mboxes_in_domain"`
	MaxQuota    int64  `json:"max_quota_for_domain"`
}

// Mailbox is a subset of a mailcow mailbox object as returned by
// GET /api/v1/get/mailbox/all. Unknown fields are ignored.
type Mailbox struct {
	Username string `json:"username"`
	Domain   string `json:"domain"`
	Name     string `json:"name"`
	Active   int    `json:"active"`
	QuotaKB  int64  `json:"quota"`
}
