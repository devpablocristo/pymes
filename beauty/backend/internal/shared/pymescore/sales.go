package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

func (c *Client) CreateSale(ctx context.Context, payload map[string]any) (map[string]any, error) {
	return pymescoreops.CreateSale(ctx, c.Client, payload)
}

func (c *Client) CreateSalePaymentLink(ctx context.Context, orgID, saleID string) (map[string]any, error) {
	return pymescoreops.CreateSalePaymentLink(ctx, c.Client, orgID, saleID)
}
