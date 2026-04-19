package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

// CreateSale creates a sale in the pymes-core.
func (c *Client) CreateSale(ctx context.Context, payload map[string]any) (map[string]any, error) {
	return pymescoreops.CreateSale(ctx, c.Client, payload)
}
