package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

func (c *Client) GetCustomer(ctx context.Context, orgID, customerID string) (map[string]any, error) {
	return pymescoreops.GetCustomer(ctx, c.Client, orgID, customerID)
}
