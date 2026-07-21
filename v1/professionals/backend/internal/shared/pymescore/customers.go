package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/core/shared/backend/pymescoreops"
)

// ResolveCustomer finds or creates a customer in the core.
func (c *Client) ResolveCustomer(ctx context.Context, tenantID, name, phone, email string) (map[string]any, error) {
	return pymescoreops.ResolveCustomer(ctx, c.Client, tenantID, name, phone, email)
}
