package pymescoreops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescorehttp"
)

func GetService(ctx context.Context, client *pymescorehttp.Client, orgID, serviceID string) (map[string]any, error) {
	result, err := client.Get(ctx, fmt.Sprintf("/v1/internal/v1/services/%s", url.PathEscape(serviceID)), orgID)
	if err != nil {
		return nil, fmt.Errorf("get service: %w", err)
	}
	return result, nil
}
