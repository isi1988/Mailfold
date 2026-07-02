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
}

// Compile-time assertion that the concrete client satisfies the interface.
var _ Mailcow = (*mailcow.Client)(nil)
