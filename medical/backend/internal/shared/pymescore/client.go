// Package pymescore expone un client HTTP hacia pymes-core para la vertical medical.
// Está preparado para que los dominios embebidos (customers, invoices, employees, etc.)
// consuman los endpoints internos de pymes-core cuando se implementen.
package pymescore

import (
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescorehttp"
)

// Client wraps the shared HTTP client for pymes-core internal API.
type Client struct {
	*pymescorehttp.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{Client: pymescorehttp.New(baseURL, token)}
}
