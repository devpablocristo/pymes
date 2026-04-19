package pymescoreops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescorehttp"
)

func ResolveCustomer(ctx context.Context, client *pymescorehttp.Client, orgID, name, phone, email string) (map[string]any, error) {
	payload := map[string]string{
		"org_id": orgID,
		"name":   name,
		"phone":  phone,
		"email":  email,
	}
	result, err := client.Post(ctx, "/v1/internal/v1/customers/resolve", orgID, payload)
	if err != nil {
		return nil, fmt.Errorf("resolve customer: %w", err)
	}
	return result, nil
}

func GetCustomer(ctx context.Context, client *pymescorehttp.Client, orgID, customerID string) (map[string]any, error) {
	result, err := client.Get(ctx, fmt.Sprintf("/v1/internal/v1/customers/%s", url.PathEscape(customerID)), orgID)
	if err != nil {
		return nil, fmt.Errorf("get customer: %w", err)
	}
	return result, nil
}
