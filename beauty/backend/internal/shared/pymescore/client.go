package pymescore

import (
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescorehttp"
)

type Client struct {
	*pymescorehttp.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{Client: pymescorehttp.New(baseURL, token)}
}
