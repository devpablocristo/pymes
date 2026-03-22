// Package pymescorehttp es el cliente HTTP compartido para llamar al API interno de pymes-core desde lambdas verticales.
package pymescorehttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) Get(ctx context.Context, path, orgID string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	c.setHeaders(req, orgID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()
	return c.decode(resp)
}

func (c *Client) Post(ctx context.Context, path, orgID string, payload any) (map[string]any, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.setHeaders(req, orgID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()
	return c.decode(resp)
}

func (c *Client) setHeaders(req *http.Request, orgID string) {
	if c.token != "" {
		req.Header.Set("X-Internal-Service-Token", c.token)
	}
	if orgID != "" {
		req.Header.Set("X-Org-ID", orgID)
	}
}

// ResolveOrgRef traduce external_id de Clerk (org_...), slug o UUID a org_id interno (vía core internal API).
func (c *Client) ResolveOrgRef(ctx context.Context, ref string) (map[string]any, error) {
	q := url.Values{}
	q.Set("ref", ref)
	return c.Get(ctx, "/v1/internal/v1/orgs/resolve-ref?"+q.Encode(), "")
}

func (c *Client) decode(resp *http.Response) (map[string]any, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("pymes-core returned %d: %s", resp.StatusCode, string(data))
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}
