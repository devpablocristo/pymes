package controlplane

import (
	"context"
	"fmt"
)

func (c *Client) CreateQuote(ctx context.Context, payload map[string]any) (map[string]any, error) {
	orgID, _ := payload["org_id"].(string)
	result, err := c.post(ctx, "/v1/internal/v1/quotes", orgID, payload)
	if err != nil {
		return nil, fmt.Errorf("create quote: %w", err)
	}
	return result, nil
}
