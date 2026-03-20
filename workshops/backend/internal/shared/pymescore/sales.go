package pymescore

import (
	"context"
	"fmt"
	"net/url"
)

func (c *Client) CreateSale(ctx context.Context, payload map[string]any) (map[string]any, error) {
	orgID, _ := payload["org_id"].(string)
	result, err := c.post(ctx, "/v1/internal/v1/sales", orgID, payload)
	if err != nil {
		return nil, fmt.Errorf("create sale: %w", err)
	}
	return result, nil
}

func (c *Client) CreateSalePaymentLink(ctx context.Context, orgID, saleID string) (map[string]any, error) {
	result, err := c.post(ctx, fmt.Sprintf("/v1/internal/v1/sales/%s/payment-link", url.PathEscape(saleID)), orgID, nil)
	if err != nil {
		return nil, fmt.Errorf("create sale payment link: %w", err)
	}
	return result, nil
}
