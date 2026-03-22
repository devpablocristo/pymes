package pymescore

import (
	"context"
	"fmt"
)

// CreateAppointment creates an appointment in the pymes-core.
func (c *Client) CreateAppointment(ctx context.Context, payload map[string]any) (map[string]any, error) {
	orgID, _ := payload["org_id"].(string)
	result, err := c.Post(ctx, "/v1/internal/v1/appointments", orgID, payload)
	if err != nil {
		return nil, fmt.Errorf("create appointment: %w", err)
	}
	return result, nil
}

// GetAppointment fetches an appointment by ID from the pymes-core.
func (c *Client) GetAppointment(ctx context.Context, orgID, id string) (map[string]any, error) {
	result, err := c.Get(ctx, fmt.Sprintf("/v1/internal/v1/appointments/%s", id), orgID)
	if err != nil {
		return nil, fmt.Errorf("get appointment: %w", err)
	}
	return result, nil
}
