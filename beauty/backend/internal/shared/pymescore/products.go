package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

func (c *Client) GetProduct(ctx context.Context, orgID, productID string) (map[string]any, error) {
	return pymescoreops.GetProduct(ctx, c.Client, orgID, productID)
}
