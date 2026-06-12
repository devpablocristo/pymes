package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/core/shared/backend/pymescoreops"
)

func (c *Client) GetBootstrap(ctx context.Context, tenantID string) (map[string]any, error) {
	return pymescoreops.GetBootstrap(ctx, c.Client, tenantID)
}
