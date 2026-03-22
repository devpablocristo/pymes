package pymescoreops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescorehttp"
)

func GetProduct(ctx context.Context, client *pymescorehttp.Client, orgID, productID string) (map[string]any, error) {
	result, err := client.Get(ctx, fmt.Sprintf("/v1/internal/v1/products/%s", url.PathEscape(productID)), orgID)
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	return result, nil
}
