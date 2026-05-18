package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

// GetBootstrap fetches the tenant bootstrap payload from pymes-core.
func (c *Client) GetBootstrap(ctx context.Context, tenantID string) (map[string]any, error) {
	return pymescoreops.GetBootstrap(ctx, c.Client, tenantID)
}

// GetSettings fetches the tenant settings from pymes-core.
func (c *Client) GetSettings(ctx context.Context, tenantID string) (map[string]any, error) {
	return pymescoreops.GetSettings(ctx, c.Client, tenantID)
}
