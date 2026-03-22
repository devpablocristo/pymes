package pymescore

import (
	"context"
	"fmt"
	"net/url"
)

func (c *Client) GetCustomer(ctx context.Context, orgID, customerID string) (map[string]any, error) {
	result, err := c.Get(ctx, fmt.Sprintf("/v1/internal/v1/customers/%s", url.PathEscape(customerID)), orgID)
	if err != nil {
		return nil, fmt.Errorf("get customer: %w", err)
	}
	return result, nil
}
