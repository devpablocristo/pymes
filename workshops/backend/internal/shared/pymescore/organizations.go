package pymescore

import (
	"context"
	"fmt"
)

func (c *Client) GetBootstrap(ctx context.Context, orgID string) (map[string]any, error) {
	result, err := c.Get(ctx, fmt.Sprintf("/v1/internal/v1/orgs/%s/bootstrap", orgID), orgID)
	if err != nil {
		return nil, fmt.Errorf("get bootstrap: %w", err)
	}
	return result, nil
}
