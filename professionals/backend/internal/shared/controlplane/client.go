// Package controlplane provides an HTTP client for calling the control-plane internal API.
package controlplane

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

// Client communicates with the control-plane backend over HTTP.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a Client pointing at the given control-plane base URL.
// token is the value sent as X-Internal-Service-Token.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) get(ctx context.Context, path string, orgID string) (map[string]any, error) {
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

func (c *Client) post(ctx context.Context, path string, orgID string, payload any) (map[string]any, error) {
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

func (c *Client) decode(resp *http.Response) (map[string]any, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("control-plane returned %d: %s", resp.StatusCode, string(data))
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}

func (c *Client) GetBusinessInfo(ctx context.Context, orgRef string) (map[string]any, error) {
	return c.get(ctx, fmt.Sprintf("/v1/public/%s/info", url.PathEscape(orgRef)), "")
}

func (c *Client) GetAvailability(ctx context.Context, orgRef string, date string, duration int) (map[string]any, error) {
	path := fmt.Sprintf("/v1/public/%s/availability", url.PathEscape(orgRef))
	query := url.Values{}
	if date != "" {
		query.Set("date", date)
	}
	if duration > 0 {
		query.Set("duration", fmt.Sprintf("%d", duration))
	}
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return c.get(ctx, path, "")
}

func (c *Client) BookAppointment(ctx context.Context, orgRef string, payload map[string]any) (map[string]any, error) {
	return c.post(ctx, fmt.Sprintf("/v1/public/%s/book", url.PathEscape(orgRef)), "", payload)
}

func (c *Client) CreateSalePaymentLink(ctx context.Context, orgID, saleID string) (map[string]any, error) {
	return c.post(ctx, fmt.Sprintf("/v1/internal/v1/sales/%s/payment-link", url.PathEscape(saleID)), orgID, nil)
}
