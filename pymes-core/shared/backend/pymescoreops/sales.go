package pymescoreops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescorehttp"
)

func CreateSale(ctx context.Context, client *pymescorehttp.Client, payload map[string]any) (map[string]any, error) {
	orgID, _ := payload["org_id"].(string)
	result, err := client.Post(ctx, "/v1/internal/v1/sales", orgID, payload)
	if err != nil {
		return nil, fmt.Errorf("create sale: %w", err)
	}
	return result, nil
}

func CreateSalePaymentLink(ctx context.Context, client *pymescorehttp.Client, orgID, saleID string) (map[string]any, error) {
	result, err := client.Post(ctx, fmt.Sprintf("/v1/internal/v1/sales/%s/payment-link", url.PathEscape(saleID)), orgID, nil)
	if err != nil {
		return nil, fmt.Errorf("create sale payment link: %w", err)
	}
	return result, nil
}
