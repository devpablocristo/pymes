package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

func (c *Client) CreateQuote(ctx context.Context, payload map[string]any) (map[string]any, error) {
	return pymescoreops.CreateQuote(ctx, c.Client, payload)
}
