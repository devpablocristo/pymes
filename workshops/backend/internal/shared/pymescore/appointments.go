package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

func (c *Client) CreateAppointment(ctx context.Context, payload map[string]any) (map[string]any, error) {
	return pymescoreops.CreateAppointment(ctx, c.Client, payload)
}
