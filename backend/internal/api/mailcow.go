package api

import (
	"context"
	"encoding/json"

	"github.com/isi1988/Mailfold/backend/internal/mailcow"
)

// Mailcow is the set of mailcow operations the HTTP layer depends on.
//
// The API layer talks to this interface rather than the concrete
// *mailcow.Client. This inverts the dependency between the two packages: the
// HTTP handlers own the contract they need, the client merely satisfies it.
// The immediate practical benefits are decoupling (the transport layer no
// longer hard-references the client implementation) and testability (handlers
// can be exercised against a fake that never performs real HTTP).
//
// Methods are grouped by resource. Add* take the mailcow "attributes" payload,
// Edit* take the affected item identifiers plus attributes, and Delete* take
// the item identifiers. Read methods either return typed values (domains,
// mailboxes, aliases) or raw JSON that is passed straight through to the client.
type Mailcow interface {
	// Domains.
	Domains(ctx context.Context) ([]mailcow.Domain, error)
	AddDomain(ctx context.Context, attr any) ([]mailcow.ActionResult, error)
	EditDomain(ctx context.Context, items []string, attr any) ([]mailcow.ActionResult, error)
	DeleteDomain(ctx context.Context, items []string) ([]mailcow.ActionResult, error)

	// Mailboxes.
	Mailboxes(ctx context.Context) ([]mailcow.Mailbox, error)
	AddMailbox(ctx context.Context, attr any) ([]mailcow.ActionResult, error)
	EditMailbox(ctx context.Context, items []string, attr any) ([]mailcow.ActionResult, error)
	DeleteMailbox(ctx context.Context, items []string) ([]mailcow.ActionResult, error)

	// Aliases.
	Aliases(ctx context.Context) ([]mailcow.Alias, error)
	AddAlias(ctx context.Context, attr any) ([]mailcow.ActionResult, error)
	EditAlias(ctx context.Context, items []string, attr any) ([]mailcow.ActionResult, error)
	DeleteAlias(ctx context.Context, items []string) ([]mailcow.ActionResult, error)

	// DKIM keys.
	DKIM(ctx context.Context, domain string) (json.RawMessage, error)
	AddDKIM(ctx context.Context, attr any) ([]mailcow.ActionResult, error)
	DeleteDKIM(ctx context.Context, items []string) ([]mailcow.ActionResult, error)

	// Sync jobs.
	SyncJobs(ctx context.Context) (json.RawMessage, error)
	AddSyncJob(ctx context.Context, attr any) ([]mailcow.ActionResult, error)
	EditSyncJob(ctx context.Context, items []string, attr any) ([]mailcow.ActionResult, error)
	DeleteSyncJob(ctx context.Context, items []string) ([]mailcow.ActionResult, error)

	// Postfix mail queue.
	MailQueue(ctx context.Context) (json.RawMessage, error)
	FlushQueue(ctx context.Context) ([]mailcow.ActionResult, error)

	// Logs.
	Logs(ctx context.Context, service string, count int) (json.RawMessage, error)

	// Fail2Ban.
	Fail2Ban(ctx context.Context) (json.RawMessage, error)
	EditFail2Ban(ctx context.Context, attr any) ([]mailcow.ActionResult, error)

	// Quarantine.
	Quarantine(ctx context.Context) (json.RawMessage, error)
	DeleteQuarantine(ctx context.Context, items []string) ([]mailcow.ActionResult, error)

	// Per-domain spam policy (allow/deny lists).
	PolicyAllow(ctx context.Context, domain string) (json.RawMessage, error)
	PolicyDeny(ctx context.Context, domain string) (json.RawMessage, error)
	AddPolicy(ctx context.Context, attr any) ([]mailcow.ActionResult, error)
	DeletePolicy(ctx context.Context, items []string) ([]mailcow.ActionResult, error)

	// System status.
	Containers(ctx context.Context) (json.RawMessage, error)
	Version(ctx context.Context) (json.RawMessage, error)
	Vmail(ctx context.Context) (json.RawMessage, error)

	// Domain admins.
	DomainAdmins(ctx context.Context) (json.RawMessage, error)
	AddDomainAdmin(ctx context.Context, attr any) ([]mailcow.ActionResult, error)
	EditDomainAdmin(ctx context.Context, items []string, attr any) ([]mailcow.ActionResult, error)
	DeleteDomainAdmin(ctx context.Context, items []string) ([]mailcow.ActionResult, error)

	// Resources (shared mailboxes / rooms).
	Resources(ctx context.Context) (json.RawMessage, error)
	AddResource(ctx context.Context, attr any) ([]mailcow.ActionResult, error)
	EditResource(ctx context.Context, items []string, attr any) ([]mailcow.ActionResult, error)
	DeleteResource(ctx context.Context, items []string) ([]mailcow.ActionResult, error)

	// App passwords (per mailbox).
	AppPasswords(ctx context.Context, mailbox string) (json.RawMessage, error)
	AddAppPassword(ctx context.Context, attr any) ([]mailcow.ActionResult, error)
	DeleteAppPassword(ctx context.Context, items []string) ([]mailcow.ActionResult, error)

	// OAuth2 clients.
	OAuth2Clients(ctx context.Context) (json.RawMessage, error)
	AddOAuth2Client(ctx context.Context, attr any) ([]mailcow.ActionResult, error)
	DeleteOAuth2Client(ctx context.Context, items []string) ([]mailcow.ActionResult, error)

	// Forwarding hosts.
	ForwardingHosts(ctx context.Context) (json.RawMessage, error)
	AddForwardingHost(ctx context.Context, attr any) ([]mailcow.ActionResult, error)
	DeleteForwardingHost(ctx context.Context, items []string) ([]mailcow.ActionResult, error)

	// Transports (sender-dependent transport maps).
	Transports(ctx context.Context) (json.RawMessage, error)
	AddTransport(ctx context.Context, attr any) ([]mailcow.ActionResult, error)
	EditTransport(ctx context.Context, items []string, attr any) ([]mailcow.ActionResult, error)
	DeleteTransport(ctx context.Context, items []string) ([]mailcow.ActionResult, error)

	// Relayhosts.
	Relayhosts(ctx context.Context) (json.RawMessage, error)
	AddRelayhost(ctx context.Context, attr any) ([]mailcow.ActionResult, error)
	EditRelayhost(ctx context.Context, items []string, attr any) ([]mailcow.ActionResult, error)
	DeleteRelayhost(ctx context.Context, items []string) ([]mailcow.ActionResult, error)

	// Outbound TLS policy maps.
	TLSPolicies(ctx context.Context) (json.RawMessage, error)
	AddTLSPolicy(ctx context.Context, attr any) ([]mailcow.ActionResult, error)
	DeleteTLSPolicy(ctx context.Context, items []string) ([]mailcow.ActionResult, error)

	// BCC maps.
	BCCMaps(ctx context.Context) (json.RawMessage, error)
	AddBCC(ctx context.Context, attr any) ([]mailcow.ActionResult, error)
	DeleteBCC(ctx context.Context, items []string) ([]mailcow.ActionResult, error)

	// Recipient maps.
	RecipientMaps(ctx context.Context) (json.RawMessage, error)
	AddRecipientMap(ctx context.Context, attr any) ([]mailcow.ActionResult, error)
	DeleteRecipientMap(ctx context.Context, items []string) ([]mailcow.ActionResult, error)
}

// Compile-time assertion that the concrete client satisfies the interface.
var _ Mailcow = (*mailcow.Client)(nil)
