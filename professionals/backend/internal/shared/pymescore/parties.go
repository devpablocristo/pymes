package pymescore

import (
	"context"
	"fmt"
)

// GetParty fetches a party by ID from the pymes-core.
func (c *Client) GetParty(ctx context.Context, orgID, partyID string) (map[string]any, error) {
	result, err := c.Get(ctx, fmt.Sprintf("/v1/internal/v1/parties/%s", partyID), orgID)
	if err != nil {
		return nil, fmt.Errorf("get party: %w", err)
	}
	return result, nil
}
