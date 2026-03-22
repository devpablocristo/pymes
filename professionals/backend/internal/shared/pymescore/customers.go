package pymescore

import (
	"context"
	"fmt"
)

// ResolveCustomer finds or creates a customer in the pymes-core.
func (c *Client) ResolveCustomer(ctx context.Context, orgID, name, phone, email string) (map[string]any, error) {
	payload := map[string]string{
		"org_id": orgID,
		"name":   name,
		"phone":  phone,
		"email":  email,
	}
	result, err := c.Post(ctx, "/v1/internal/v1/customers/resolve", orgID, payload)
	if err != nil {
		return nil, fmt.Errorf("resolve customer: %w", err)
	}
	return result, nil
}
