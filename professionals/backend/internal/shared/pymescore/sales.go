package pymescore

import (
	"context"
	"fmt"
)

// CreateSale creates a sale in the pymes-core.
func (c *Client) CreateSale(ctx context.Context, payload map[string]any) (map[string]any, error) {
	orgID, _ := payload["org_id"].(string)
	result, err := c.Post(ctx, "/v1/internal/v1/sales", orgID, payload)
	if err != nil {
		return nil, fmt.Errorf("create sale: %w", err)
	}
	return result, nil
}
