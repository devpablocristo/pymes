package controlplane

import (
	"context"
	"fmt"
	"net/url"
)

func (c *Client) GetParty(ctx context.Context, orgID, partyID string) (map[string]any, error) {
	result, err := c.get(ctx, fmt.Sprintf("/v1/internal/v1/parties/%s", url.PathEscape(partyID)), orgID)
	if err != nil {
		return nil, fmt.Errorf("get party: %w", err)
	}
	return result, nil
}
