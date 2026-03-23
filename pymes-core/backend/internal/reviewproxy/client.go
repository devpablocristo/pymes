// Package reviewproxy proxies policy/approval requests from the frontend to Nexus Review.
package reviewproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client es un cliente HTTP para Nexus Review API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient crea un nuevo cliente para Nexus Review.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// doRequest ejecuta una petición HTTP al API de Review y retorna body + status.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) ([]byte, int, error) {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, 0, fmt.Errorf("create request %s %s: %w", method, path, err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response %s %s: %w", method, path, err)
	}

	return data, resp.StatusCode, nil
}

// ListPolicies lista las políticas activas.
func (c *Client) ListPolicies(ctx context.Context) ([]byte, int, error) {
	return c.doRequest(ctx, http.MethodGet, "/v1/policies", nil)
}

// CreatePolicy crea una nueva política.
func (c *Client) CreatePolicy(ctx context.Context, body io.Reader) ([]byte, int, error) {
	return c.doRequest(ctx, http.MethodPost, "/v1/policies", body)
}

// UpdatePolicy actualiza una política existente.
func (c *Client) UpdatePolicy(ctx context.Context, id string, body io.Reader) ([]byte, int, error) {
	return c.doRequest(ctx, http.MethodPatch, "/v1/policies/"+id, body)
}

// DeletePolicy elimina una política.
func (c *Client) DeletePolicy(ctx context.Context, id string) (int, error) {
	_, status, err := c.doRequest(ctx, http.MethodDelete, "/v1/policies/"+id, nil)
	return status, err
}

// ListActionTypes lista los action types registrados.
func (c *Client) ListActionTypes(ctx context.Context) ([]byte, int, error) {
	return c.doRequest(ctx, http.MethodGet, "/v1/action-types", nil)
}

// ListPendingApprovals lista aprobaciones pendientes.
func (c *Client) ListPendingApprovals(ctx context.Context) ([]byte, int, error) {
	return c.doRequest(ctx, http.MethodGet, "/v1/approvals/pending", nil)
}

// Approve aprueba una solicitud.
func (c *Client) Approve(ctx context.Context, id string, body io.Reader) ([]byte, int, error) {
	return c.doRequest(ctx, http.MethodPost, "/v1/approvals/"+id+"/approve", body)
}

// Reject rechaza una solicitud.
func (c *Client) Reject(ctx context.Context, id string, body io.Reader) ([]byte, int, error) {
	return c.doRequest(ctx, http.MethodPost, "/v1/approvals/"+id+"/reject", body)
}
