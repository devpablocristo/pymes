// Package pymescorehttp es el cliente HTTP compartido para llamar al API interno de pymes-core desde lambdas verticales.
package pymescorehttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/devpablocristo/core/http/go/httpclient"
)

type Client struct {
	caller *httpclient.Caller
	token  string
}

func New(baseURL, token string) *Client {
	return &Client{
		caller: &httpclient.Caller{
			HTTP:    &http.Client{Timeout: 10 * time.Second},
			BaseURL: baseURL,
		},
		token: token,
	}
}

func (c *Client) Get(ctx context.Context, path, orgID string) (map[string]any, error) {
	opts := c.headerOpts(orgID)
	status, body, err := c.caller.DoJSON(ctx, http.MethodGet, path, nil, opts...)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	return c.decode(status, body)
}

func (c *Client) Post(ctx context.Context, path, orgID string, payload any) (map[string]any, error) {
	opts := c.headerOpts(orgID)
	status, body, err := c.caller.DoJSON(ctx, http.MethodPost, path, payload, opts...)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	return c.decode(status, body)
}

// ResolveOrgRef traduce external_id de Clerk (org_...), slug o UUID a org_id interno (vía core internal API).
func (c *Client) ResolveOrgRef(ctx context.Context, ref string) (map[string]any, error) {
	q := url.Values{}
	q.Set("ref", ref)
	return c.Get(ctx, "/v1/internal/v1/orgs/resolve-ref?"+q.Encode(), "")
}

func (c *Client) headerOpts(orgID string) []httpclient.RequestOption {
	var opts []httpclient.RequestOption
	if c.token != "" {
		opts = append(opts, httpclient.WithHeader("X-Internal-Service-Token", c.token))
	}
	if orgID != "" {
		opts = append(opts, httpclient.WithHeader("X-Org-ID", orgID))
	}
	return opts
}

func (c *Client) decode(status int, body []byte) (map[string]any, error) {
	if status >= 400 {
		return nil, fmt.Errorf("pymes-core returned %d: %s", status, string(body))
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}
