package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

func (c *Client) GetService(ctx context.Context, orgID, serviceID string) (map[string]any, error) {
	return pymescoreops.GetService(ctx, c.Client, orgID, serviceID)
}
