// Package ports defines the worker application's boundaries.
package ports

import (
	"context"

	workerdomain "github.com/devpablocristo/pymes/v3/backend/internal/worker/domain"
)

// Dispatcher is the application port driven by the worker scheduler.
type Dispatcher interface {
	DispatchOnce(context.Context) error
}

// Readiness is the durable dependency check exposed by /readyz.
type Readiness interface {
	Ready(context.Context) error
}

// MetricsReader is the outbound operational-data port.
type MetricsReader interface {
	Collect(context.Context) (workerdomain.Metrics, error)
}

// CircuitState exposes only the dependency state needed by operations.
type CircuitState interface {
	CircuitOpen() bool
}

// EventTrace carries only the durable W3C context needed to continue an
// outbox event trace. Payload and tenant data deliberately stay outside it.
type EventTrace struct {
	Name        string
	TraceParent string
}

// EventSpan is independent from any telemetry SDK.
type EventSpan interface {
	End(error)
}

// EventTracer is the seam for continuing durable event traces when outbox
// envelopes persist traceparent in a later change.
type EventTracer interface {
	ContinueEvent(context.Context, EventTrace) (context.Context, EventSpan)
}
