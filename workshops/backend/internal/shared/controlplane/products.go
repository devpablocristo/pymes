package controlplane

import (
	"context"
	"fmt"
	"net/url"
)

func (c *Client) GetProduct(ctx context.Context, orgID, productID string) (map[string]any, error) {
	result, err := c.get(ctx, fmt.Sprintf("/v1/internal/v1/products/%s", url.PathEscape(productID)), orgID)
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	return result, nil
}
