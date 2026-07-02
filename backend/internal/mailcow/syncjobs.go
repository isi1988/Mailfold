package mailcow

import (
	"context"
	"encoding/json"
)

// SyncJobs returns every configured sync job as raw JSON passthrough. The
// endpoint is requested with "no_log" and "hide_pw=1" so the response omits
// verbose run logs and never exposes stored account passwords. The payload is
// returned untouched because the frontend renders the sync-job list directly and
// its shape is large and only partially stable.
func (c *Client) SyncJobs(ctx context.Context) (json.RawMessage, error) {
	return c.rawGet(ctx, "/api/v1/get/syncjobs/all/no_log?hide_pw=1")
}

// AddSyncJob creates a new IMAP sync job from the attributes in attr (source
// server, credentials, target mailbox, and so on). It delegates to action so all
// mutating calls share one request/response implementation.
func (c *Client) AddSyncJob(ctx context.Context, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/add/syncjob", attr)
}

// EditSyncJob modifies one or more sync jobs, applying attr to each identifier in
// items. The identifiers and attributes are wrapped in the standard EditRequest
// shape that mailcow's edit endpoints require.
func (c *Client) EditSyncJob(ctx context.Context, items []string, attr any) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/edit/syncjob", EditRequest{Items: items, Attr: attr})
}

// DeleteSyncJob removes one or more sync jobs identified by id. mailcow expects a
// bare array of identifiers for deletion, so items is forwarded unchanged.
func (c *Client) DeleteSyncJob(ctx context.Context, items []string) ([]ActionResult, error) {
	return c.action(ctx, "/api/v1/delete/syncjob", items)
}
