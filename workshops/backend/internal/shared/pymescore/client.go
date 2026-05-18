package pymescore

import (
	"github.com/devpablocristo/pymes/core/shared/backend/pymescorehttp"
)

// Client wraps the shared HTTP client for core internal API.
type Client struct {
	*pymescorehttp.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{Client: pymescorehttp.New(baseURL, token)}
}
