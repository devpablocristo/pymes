// Package pymescore provides an HTTP client for calling the core internal API.
package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/core/shared/backend/pymescorehttp"
	"github.com/devpablocristo/pymes/core/shared/backend/pymescoreops"
)

type AvailabilityParams = pymescoreops.AvailabilityParams

// Client communicates with the core backend over HTTP.
type Client struct {
	*pymescorehttp.Client
}

// NewClient creates a Client pointing at the given core base URL.
func NewClient(baseURL, token string) *Client {
	return &Client{Client: pymescorehttp.New(baseURL, token)}
}

func (c *Client) GetBusinessInfo(ctx context.Context, tenantRef string) (map[string]any, error) {
	return pymescoreops.GetBusinessInfo(ctx, c.Client, tenantRef)
}

func (c *Client) GetAvailability(ctx context.Context, tenantRef string, params AvailabilityParams) (map[string]any, error) {
	return pymescoreops.GetAvailability(ctx, c.Client, tenantRef, params)
}

func (c *Client) BookScheduling(ctx context.Context, tenantRef string, payload map[string]any) (map[string]any, error) {
	return pymescoreops.BookScheduling(ctx, c.Client, tenantRef, payload)
}

func (c *Client) CreateSalePaymentLink(ctx context.Context, tenantID, saleID string) (map[string]any, error) {
	return pymescoreops.CreateSalePaymentLink(ctx, c.Client, tenantID, saleID)
}
