package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

func (c *Client) GetBusinessInfo(ctx context.Context, tenantRef string) (map[string]any, error) {
	return pymescoreops.GetBusinessInfo(ctx, c.Client, tenantRef)
}

func (c *Client) BookScheduling(ctx context.Context, tenantRef string, payload map[string]any) (map[string]any, error) {
	return pymescoreops.BookScheduling(ctx, c.Client, tenantRef, payload)
}
