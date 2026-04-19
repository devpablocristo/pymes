package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

func (c *Client) GetParty(ctx context.Context, orgID, partyID string) (map[string]any, error) {
	return pymescoreops.GetParty(ctx, c.Client, orgID, partyID)
}
