package pymescoreops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescorehttp"
)

func GetParty(ctx context.Context, client *pymescorehttp.Client, orgID, partyID string) (map[string]any, error) {
	result, err := client.Get(ctx, fmt.Sprintf("/v1/internal/v1/parties/%s", url.PathEscape(partyID)), orgID)
	if err != nil {
		return nil, fmt.Errorf("get party: %w", err)
	}
	return result, nil
}
