package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/core/shared/backend/pymescoreops"
)

func (c *Client) GetProduct(ctx context.Context, tenantID, productID string) (map[string]any, error) {
	return pymescoreops.GetProduct(ctx, c.Client, tenantID, productID)
}
