package pymescore

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// ListProducts queries the pymes-core product catalog.
func (c *Client) ListProducts(ctx context.Context, orgID string, query string, limit int) (map[string]any, error) {
	params := url.Values{}
	if query != "" {
		params.Set("q", query)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	path := "/v1/internal/v1/products"
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}
	result, err := c.get(ctx, path, orgID)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	return result, nil
}
