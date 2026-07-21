package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/core/shared/backend/pymescoreops"
)

func (c *Client) GetService(ctx context.Context, tenantID, serviceID string) (map[string]any, error) {
	return pymescoreops.GetService(ctx, c.Client, tenantID, serviceID)
}

type CoreService = pymescoreops.CoreService

func (c *Client) ListPublicServices(ctx context.Context, tenantRef, vertical, segment, search string) ([]CoreService, error) {
	return pymescoreops.ListPublicServices(ctx, c.Client, tenantRef, vertical, segment, search)
}
