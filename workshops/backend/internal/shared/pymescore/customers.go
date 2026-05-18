package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/core/shared/backend/pymescoreops"
)

func (c *Client) GetCustomer(ctx context.Context, tenantID, customerID string) (map[string]any, error) {
	return pymescoreops.GetCustomer(ctx, c.Client, tenantID, customerID)
}
