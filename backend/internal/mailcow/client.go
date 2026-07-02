// Package mailcow is a thin, typed client for the mailcow REST API.
package mailcow

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxResponseBytes = 16 << 20 // 16 MiB cap on API responses.

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
		http:    &http.Client{Timeout: 30 * time.Second, Transport: transport},
	}
}

// get performs an authenticated GET and decodes the JSON body into out.
func (c *Client) get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

// post performs an authenticated POST with a JSON body and decodes the JSON
// response into out. out may be nil to ignore the response body.
func (c *Client) post(ctx context.Context, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	return c.do(ctx, http.MethodPost, path, reader, out)
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mailcow %s %s: status %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("mailcow %s %s: decode response: %w", method, path, err)
	}
	return nil
}

// action performs a mailcow add/edit/delete request and returns the resulting
// action-result array. Every mutating resource method delegates here so the
// request/response handling lives in exactly one place.
func (c *Client) action(ctx context.Context, path string, body any) ([]ActionResult, error) {
	var out []ActionResult
	if err := c.post(ctx, path, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// rawGet performs a mailcow GET and returns the upstream JSON untouched. It
// backs every read-only endpoint whose payload the API layer passes straight
// through to the frontend (status, logs, queue, quarantine, policy, ...).
func (c *Client) rawGet(ctx context.Context, path string) (json.RawMessage, error) {
	var out json.RawMessage
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return out, nil
}
