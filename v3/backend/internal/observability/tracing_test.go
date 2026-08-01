package observability

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestTransportPropagatesOnlyW3CTraceContext(t *testing.T) {
	t.Parallel()
	traceID, _ := trace.TraceIDFromHex("00112233445566778899aabbccddeeff")
	spanID, _ := trace.SpanIDFromHex("0011223344556677")
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), trace.NewSpanContext(
		trace.SpanContextConfig{
			TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled, Remote: true,
		},
	))
	transport := Transport(roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if got := request.Header.Get("traceparent"); got != "00-00112233445566778899aabbccddeeff-0011223344556677-01" {
			t.Fatalf("traceparent=%q", got)
		}
		if got := request.Header.Get("baggage"); got != "" {
			t.Fatalf("baggage must not cross the service boundary: %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	}))
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://accounting.internal/path", strings.NewReader("{}"))
	request.Header.Set("baggage", "customer_name=secret")
	if _, err := transport.RoundTrip(request); err != nil {
		t.Fatal(err)
	}
}

func TestConfigureTracingRejectsUnsafeConfiguration(t *testing.T) {
	t.Parallel()
	_, err := ConfigureTracing(context.Background(), "api", "test", func(key string) string {
		if key == "PYMES_TRACE_SAMPLE_RATIO" {
			return "2"
		}
		return ""
	})
	if err == nil {
		t.Fatal("invalid sampling ratio must fail closed")
	}
}
