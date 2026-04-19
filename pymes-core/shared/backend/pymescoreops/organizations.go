package pymescoreops

import (
	"context"
	"fmt"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescorehttp"
)

func GetBootstrap(ctx context.Context, client *pymescorehttp.Client, orgID string) (map[string]any, error) {
	result, err := client.Get(ctx, fmt.Sprintf("/v1/internal/v1/orgs/%s/bootstrap", orgID), orgID)
	if err != nil {
		return nil, fmt.Errorf("get bootstrap: %w", err)
	}
	return result, nil
}

func GetSettings(ctx context.Context, client *pymescorehttp.Client, orgID string) (map[string]any, error) {
	result, err := client.Get(ctx, fmt.Sprintf("/v1/internal/v1/orgs/%s/settings", orgID), orgID)
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}
	return result, nil
}
