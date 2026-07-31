package companion

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestCircuitBreakerOpensAndRecoversAfterCooldown(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	calls := 0
	fail := true
	breaker := &circuitBreakerTransport{
		base: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			calls++
			if fail {
				return nil, errors.New("dependency unavailable")
			}
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
		}),
		threshold: 2,
		openFor:   time.Minute,
		now:       func() time.Time { return now },
	}
	request, _ := http.NewRequest(http.MethodGet, "http://dependency/healthz", nil)
	for range 2 {
		if _, err := breaker.RoundTrip(request); err == nil {
			t.Fatal("expected dependency failure")
		}
	}
	if _, err := breaker.RoundTrip(request); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected open circuit, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("open circuit called dependency %d times", calls)
	}
	now = now.Add(time.Minute)
	fail = false
	if _, err := breaker.RoundTrip(request); err != nil {
		t.Fatalf("expected successful recovery probe, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected one recovery probe, got %d calls", calls)
	}
}

func TestNewServiceHTTPClientHasBoundedTimeout(t *testing.T) {
	t.Parallel()
	client := NewServiceHTTPClient()
	if client.Timeout() <= 0 {
		t.Fatal("service client must have a timeout")
	}
}
