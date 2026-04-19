package pymescore

import (
	"context"
	"fmt"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

// CreateBooking creates a booking in the pymes-core scheduling module.
func (c *Client) CreateBooking(ctx context.Context, payload map[string]any) (map[string]any, error) {
	return pymescoreops.CreateBooking(ctx, c.Client, payload)
}

// GetBooking fetches a booking by ID from the pymes-core scheduling module.
func (c *Client) GetBooking(ctx context.Context, orgID, id string) (map[string]any, error) {
	result, err := c.Get(ctx, fmt.Sprintf("/v1/internal/v1/scheduling/bookings/%s", id), orgID)
	if err != nil {
		return nil, fmt.Errorf("get booking: %w", err)
	}
	return result, nil
}
