package mailcow

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// Logs returns the most recent raw log entries for a single mailcow service
// (for example "postfix", "dovecot", "rspamd" or "sogo").
//
// It issues GET /api/v1/get/logs/{service}/{count} and forwards the upstream
// JSON untouched as json.RawMessage. The logs are returned verbatim because
// their structure differs per service and the consumer only displays them, so
// there is nothing to gain from decoding into a typed value here. The path is
// assembled with fmt.Sprintf, and strconv.Itoa is used to render the numeric
// count as a path segment; the actual GET is delegated to rawGet so that
// transport concerns stay in client.go.
func (c *Client) Logs(ctx context.Context, service string, count int) (json.RawMessage, error) {
	path := fmt.Sprintf("/api/v1/get/logs/%s/%s", service, strconv.Itoa(count))
	return c.rawGet(ctx, path)
}
