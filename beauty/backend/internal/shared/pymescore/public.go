package pymescore

import (
	"context"
	"fmt"
	"net/url"
)

func (c *Client) GetBusinessInfo(ctx context.Context, orgRef string) (map[string]any, error) {
	return c.get(ctx, fmt.Sprintf("/v1/public/%s/info", url.PathEscape(orgRef)), "")
}

func (c *Client) BookAppointment(ctx context.Context, orgRef string, payload map[string]any) (map[string]any, error) {
	return c.post(ctx, fmt.Sprintf("/v1/public/%s/book", url.PathEscape(orgRef)), "", payload)
}
