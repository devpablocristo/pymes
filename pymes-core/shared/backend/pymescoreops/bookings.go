package pymescoreops

import (
	"context"
	"fmt"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescorehttp"
)

func CreateBooking(ctx context.Context, client *pymescorehttp.Client, payload map[string]any) (map[string]any, error) {
	orgID, _ := payload["org_id"].(string)
	result, err := client.Post(ctx, "/v1/internal/v1/scheduling/bookings", orgID, payload)
	if err != nil {
		return nil, fmt.Errorf("create booking: %w", err)
	}
	return result, nil
}
