package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

func (c *Client) CreateBooking(ctx context.Context, payload map[string]any) (map[string]any, error) {
	return pymescoreops.CreateBooking(ctx, c.Client, payload)
}
