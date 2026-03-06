package controlplane

import (
	"context"
	"fmt"
	"strconv"
)

// ListProducts queries the control-plane product catalog.
func (c *Client) ListProducts(ctx context.Context, orgID string, query string, limit int) (map[string]any, error) {
	path := fmt.Sprintf("/v1/internal/v1/products?q=%s&limit=%s", query, strconv.Itoa(limit))
	result, err := c.get(ctx, path, orgID)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	return result, nil
}
