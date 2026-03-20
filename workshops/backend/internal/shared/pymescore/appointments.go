package pymescore

import (
	"context"
	"fmt"
)

func (c *Client) CreateAppointment(ctx context.Context, payload map[string]any) (map[string]any, error) {
	orgID, _ := payload["org_id"].(string)
	result, err := c.post(ctx, "/v1/internal/v1/appointments", orgID, payload)
	if err != nil {
		return nil, fmt.Errorf("create appointment: %w", err)
	}
	return result, nil
}
