// architecture:adapter external
package commerce

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	servicehttphelpers "github.com/devpablocristo/pymes/v3/backend/internal/commerce/service_http/helpers"
	"github.com/devpablocristo/pymes/v3/backend/internal/observability"
)

var ErrCircuitOpen = errors.New("dependency circuit is open")

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// InternalTokenSource and PlatformTokenSource are owned by the private HTTP
// clients that consume them. Identity adapters satisfy these narrow ports.
type InternalTokenSource interface {
	Token(context.Context, string, string) (string, error)
}

type PlatformTokenSource interface {
	PlatformToken(context.Context, string) (string, error)
}

type ServiceHTTPClient struct {
	client  *http.Client
	breaker *circuitBreakerTransport
}

// NewServiceHTTPClient creates a bounded client for one private dependency.
// Callers must keep one instance per dependency so Fiscal and Accounting have
// independent failure state.
func NewServiceHTTPClient() *ServiceHTTPClient {
	settings := servicehttphelpers.DefaultSettings()
	breaker := &circuitBreakerTransport{
		base:      http.DefaultTransport,
		threshold: settings.FailureThreshold,
		openFor:   settings.OpenFor,
		now:       time.Now,
	}
	return &ServiceHTTPClient{client: &http.Client{
		Timeout:   settings.RequestTimeout,
		Transport: observability.Transport(breaker),
	}, breaker: breaker}
}

func (c *ServiceHTTPClient) Do(request *http.Request) (*http.Response, error) {
	return c.client.Do(request)
}

func (c *ServiceHTTPClient) Timeout() time.Duration { return c.client.Timeout }

func (c *ServiceHTTPClient) CircuitOpen() bool { return c.breaker.open() }

func (t *circuitBreakerTransport) open() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return servicehttphelpers.CircuitOpen(
		t.openedAt,
		t.now().UTC(),
		t.openFor,
	)
}

type circuitBreakerTransport struct {
	base      http.RoundTripper
	threshold int
	openFor   time.Duration
	now       func() time.Time

	mu       sync.Mutex
	failures int
	openedAt time.Time
}

func (t *circuitBreakerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if t.base == nil {
		t.base = http.DefaultTransport
	}
	if t.threshold < 1 {
		t.threshold = 1
	}
	if t.now == nil {
		t.now = time.Now
	}
	now := t.now().UTC()
	t.mu.Lock()
	if !t.openedAt.IsZero() && now.Sub(t.openedAt) < t.openFor {
		t.mu.Unlock()
		return nil, ErrCircuitOpen
	}
	if !t.openedAt.IsZero() {
		// Allow a probe after the cool-down. A failed probe opens the circuit
		// immediately; a successful one resets it.
		t.failures = t.threshold - 1
		t.openedAt = time.Time{}
	}
	t.mu.Unlock()

	response, err := t.base.RoundTrip(request)
	failed := err != nil || response.StatusCode >= http.StatusInternalServerError
	t.mu.Lock()
	defer t.mu.Unlock()
	if !failed {
		t.failures = 0
		t.openedAt = time.Time{}
		return response, err
	}
	t.failures++
	if t.failures >= t.threshold {
		t.openedAt = now
	}
	return response, err
}
