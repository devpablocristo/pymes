package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

func (c *Client) GetBootstrap(ctx context.Context, orgID string) (map[string]any, error) {
	return pymescoreops.GetBootstrap(ctx, c.Client, orgID)
}
