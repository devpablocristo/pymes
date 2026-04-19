package pymescore

import (
	"context"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

func (c *Client) GetBusinessInfo(ctx context.Context, orgRef string) (map[string]any, error) {
	return pymescoreops.GetBusinessInfo(ctx, c.Client, orgRef)
}

func (c *Client) BookScheduling(ctx context.Context, orgRef string, payload map[string]any) (map[string]any, error) {
	return pymescoreops.BookScheduling(ctx, c.Client, orgRef, payload)
}
