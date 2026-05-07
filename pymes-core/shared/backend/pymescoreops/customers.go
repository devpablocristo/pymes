package pymescoreops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescorehttp"
)

func ResolveCustomer(ctx context.Context, client *pymescorehttp.Client, tenantID, name, phone, email string) (map[string]any, error) {
	payload := map[string]string{
		"tenant_id": tenantID,
		"name":      name,
		"phone":     phone,
		"email":     email,
	}
	result, err := client.Post(ctx, "/v1/internal/v1/customers/resolve", tenantID, payload)
	if err != nil {
		return nil, fmt.Errorf("resolve customer: %w", err)
	}
	return result, nil
}

func GetCustomer(ctx context.Context, client *pymescorehttp.Client, tenantID, customerID string) (map[string]any, error) {
	result, err := client.Get(ctx, fmt.Sprintf("/v1/internal/v1/customers/%s", url.PathEscape(customerID)), tenantID)
	if err != nil {
		return nil, fmt.Errorf("get customer: %w", err)
	}
	return result, nil
}
