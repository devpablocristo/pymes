package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

// GetBootstrap fetches the organization bootstrap payload from pymes-core.
func (c *Client) GetBootstrap(ctx context.Context, orgID string) (map[string]any, error) {
	return pymescoreops.GetBootstrap(ctx, c.Client, orgID)
}

// GetSettings fetches the tenant settings from pymes-core.
func (c *Client) GetSettings(ctx context.Context, orgID string) (map[string]any, error) {
	return pymescoreops.GetSettings(ctx, c.Client, orgID)
}
