package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

func (c *Client) GetService(ctx context.Context, orgID, serviceID string) (map[string]any, error) {
	return pymescoreops.GetService(ctx, c.Client, orgID, serviceID)
}

type CoreService = pymescoreops.CoreService

func (c *Client) ListPublicServices(ctx context.Context, orgRef, vertical, segment, search string) ([]CoreService, error) {
	return pymescoreops.ListPublicServices(ctx, c.Client, orgRef, vertical, segment, search)
}
