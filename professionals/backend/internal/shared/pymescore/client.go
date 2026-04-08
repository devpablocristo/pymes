// Package pymescore provides an HTTP client for calling the pymes-core internal API.
package pymescore

import (
	"context"
	"fmt"
	"net/url"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescorehttp"
)

type AvailabilityParams struct {
	Date       string
	Duration   int
	BranchID   string
	ServiceID  string
	ResourceID string
}

// Client communicates with the pymes-core backend over HTTP.
type Client struct {
	*pymescorehttp.Client
}

// NewClient creates a Client pointing at the given pymes-core base URL.
func NewClient(baseURL, token string) *Client {
	return &Client{Client: pymescorehttp.New(baseURL, token)}
}

func (c *Client) GetBusinessInfo(ctx context.Context, orgRef string) (map[string]any, error) {
	return c.Get(ctx, fmt.Sprintf("/v1/public/%s/info", url.PathEscape(orgRef)), "")
}

func (c *Client) GetAvailability(ctx context.Context, orgRef string, params AvailabilityParams) (map[string]any, error) {
	path := fmt.Sprintf("/v1/public/%s/availability", url.PathEscape(orgRef))
	query := url.Values{}
	if params.Date != "" {
		query.Set("date", params.Date)
	}
	if params.Duration > 0 {
		query.Set("duration", fmt.Sprintf("%d", params.Duration))
	}
	if params.BranchID != "" {
		query.Set("branch_id", params.BranchID)
	}
	if params.ServiceID != "" {
		query.Set("service_id", params.ServiceID)
	}
	if params.ResourceID != "" {
		query.Set("resource_id", params.ResourceID)
	}
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return c.Get(ctx, path, "")
}

func (c *Client) BookScheduling(ctx context.Context, orgRef string, payload map[string]any) (map[string]any, error) {
	return c.Post(ctx, fmt.Sprintf("/v1/public/%s/book", url.PathEscape(orgRef)), "", payload)
}

func (c *Client) CreateSalePaymentLink(ctx context.Context, orgID, saleID string) (map[string]any, error) {
	return c.Post(ctx, fmt.Sprintf("/v1/internal/v1/sales/%s/payment-link", url.PathEscape(saleID)), orgID, nil)
}
