package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

// ResolveCustomer finds or creates a customer in the pymes-core.
func (c *Client) ResolveCustomer(ctx context.Context, orgID, name, phone, email string) (map[string]any, error) {
	return pymescoreops.ResolveCustomer(ctx, c.Client, orgID, name, phone, email)
}
