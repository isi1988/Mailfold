// Package mailcow is a thin, typed client for the mailcow REST API.
package mailcow

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxResponseBytes = 8 << 20 // 8 MiB cap on API responses.

// Client wraps the mailcow HTTP API with API-key authentication.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// NewClient creates a mailcow API client. When insecureTLS is true, TLS
// certificate verification is skipped (development only).
func NewClient(baseURL, apiKey string, insecureTLS bool) *Client {
	transport := &http.Transport{}
	if insecureTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in dev flag
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 15 * time.Second, Transport: transport},
	}
}

// get performs an authenticated GET and decodes the JSON body into out.
func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mailcow GET %s: status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("mailcow GET %s: decode response: %w", path, err)
	}
	return nil
}

// Domains returns all mail domains configured in mailcow.
func (c *Client) Domains(ctx context.Context) ([]Domain, error) {
	var domains []Domain
	if err := c.get(ctx, "/api/v1/get/domain/all", &domains); err != nil {
		return nil, err
	}
	return domains, nil
}

// Mailboxes returns all mailboxes configured in mailcow.
func (c *Client) Mailboxes(ctx context.Context) ([]Mailbox, error) {
	var mailboxes []Mailbox
	if err := c.get(ctx, "/api/v1/get/mailbox/all", &mailboxes); err != nil {
		return nil, err
	}
	return mailboxes, nil
}
