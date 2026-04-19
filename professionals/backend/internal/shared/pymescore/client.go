// Package pymescore provides an HTTP client for calling the pymes-core internal API.
package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescorehttp"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

type AvailabilityParams = pymescoreops.AvailabilityParams

// Client communicates with the pymes-core backend over HTTP.
type Client struct {
	*pymescorehttp.Client
}

// NewClient creates a Client pointing at the given pymes-core base URL.
func NewClient(baseURL, token string) *Client {
	return &Client{Client: pymescorehttp.New(baseURL, token)}
}

func (c *Client) GetBusinessInfo(ctx context.Context, orgRef string) (map[string]any, error) {
	return pymescoreops.GetBusinessInfo(ctx, c.Client, orgRef)
}

func (c *Client) GetAvailability(ctx context.Context, orgRef string, params AvailabilityParams) (map[string]any, error) {
	return pymescoreops.GetAvailability(ctx, c.Client, orgRef, params)
}

func (c *Client) BookScheduling(ctx context.Context, orgRef string, payload map[string]any) (map[string]any, error) {
	return pymescoreops.BookScheduling(ctx, c.Client, orgRef, payload)
}

func (c *Client) CreateSalePaymentLink(ctx context.Context, orgID, saleID string) (map[string]any, error) {
	return pymescoreops.CreateSalePaymentLink(ctx, c.Client, orgID, saleID)
}
