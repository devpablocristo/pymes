package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/core/shared/backend/pymescoreops"
)

// GetParty fetches a party by ID from the core.
func (c *Client) GetParty(ctx context.Context, tenantID, partyID string) (map[string]any, error) {
	return pymescoreops.GetParty(ctx, c.Client, tenantID, partyID)
}
