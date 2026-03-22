package pymescore

import (
	"context"
	"fmt"
	"net/url"
)

func (c *Client) GetBusinessInfo(ctx context.Context, orgRef string) (map[string]any, error) {
	return c.Get(ctx, fmt.Sprintf("/v1/public/%s/info", url.PathEscape(orgRef)), "")
}

func (c *Client) BookAppointment(ctx context.Context, orgRef string, payload map[string]any) (map[string]any, error) {
	return c.Post(ctx, fmt.Sprintf("/v1/public/%s/book", url.PathEscape(orgRef)), "", payload)
}
