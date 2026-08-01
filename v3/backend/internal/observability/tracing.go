// architecture:adapter external
package observability

import (
	"context"
	"fmt"
	"net/http"
	"os"

	platformobservability "github.com/devpablocristo/platform/observability/go"
	tracinghelpers "github.com/devpablocristo/pymes/v3/backend/internal/observability/tracing/helpers"
	tracingmodels "github.com/devpablocristo/pymes/v3/backend/internal/observability/tracing/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/devpablocristo/pymes/v3/backend"

// ConfigureTracing reuses the published Platform tracer provider while
// retaining exporter selection at the deployment edge.
func ConfigureTracing(
	ctx context.Context,
	serviceName, environment string,
	getenv func(string) string,
) (func(context.Context) error, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	settings, err := tracinghelpers.SettingsFromEnv(serviceName, environment, getenv)
	if err != nil {
		return nil, err
	}
	return platformobservability.NewTracerProvider(ctx, platformobservability.TracingConfig{
		ServiceName:    settings.ServiceName,
		ServiceVersion: settings.ServiceVersion,
		Environment:    settings.Environment,
		Exporter:       settings.Exporter,
		OTLPEndpoint:   settings.OTLPEndpoint,
		OTLPInsecure:   settings.OTLPInsecure,
		SampleRatio:    settings.SampleRatio,
	})
}

func startServerSpan(r *http.Request) (*http.Request, trace.Span) {
	ctx := propagation.TraceContext{}.Extract(
		r.Context(),
		propagation.HeaderCarrier(r.Header),
	)
	ctx, span := platformobservability.Tracer(instrumentationName).Start(
		ctx,
		r.Method,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(attribute.String("http.request.method", r.Method)),
	)
	return r.WithContext(ctx), span
}

func finishServerSpan(span trace.Span, method, route string, status int) {
	if span == nil {
		return
	}
	span.SetName(method + " " + route)
	span.SetAttributes(
		attribute.String("http.route", route),
		attribute.Int("http.response.status_code", status),
	)
	if status >= http.StatusInternalServerError {
		span.SetStatus(codes.Error, "server error")
	}
	span.End()
}

func traceIDs(ctx context.Context) (string, string) {
	return platformobservability.TraceIDFromContext(ctx),
		platformobservability.SpanIDFromContext(ctx)
}

// Transport wraps internal HTTP calls with W3C trace-context propagation. It
// deliberately excludes baggage so arbitrary user metadata cannot cross
// private service boundaries.
func Transport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return tracingRoundTripper{base: base}
}

type tracingRoundTripper struct {
	base http.RoundTripper
}

func (t tracingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil {
		return nil, fmt.Errorf("nil HTTP request")
	}
	ctx, span := otel.Tracer(instrumentationName).Start(
		request.Context(),
		request.Method+" internal",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("http.request.method", request.Method),
			attribute.String("server.address", request.URL.Hostname()),
		),
	)
	defer span.End()
	cloned := request.Clone(ctx)
	cloned.Header = request.Header.Clone()
	cloned.Header.Del("baggage")
	propagation.TraceContext{}.Inject(
		ctx,
		propagation.HeaderCarrier(cloned.Header),
	)
	response, err := t.base.RoundTrip(cloned)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "transport error")
		return nil, err
	}
	span.SetAttributes(attribute.Int("http.response.status_code", response.StatusCode))
	if response.StatusCode >= http.StatusInternalServerError {
		span.SetStatus(codes.Error, "server error")
	}
	return response, nil
}

func defaultTracingValue(value, fallback string) string {
	return tracinghelpers.DefaultValue(value, fallback)
}

var _ tracingmodels.Settings
