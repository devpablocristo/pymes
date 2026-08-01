// Package usecases contains the worker's application orchestration.
package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	workerdomain "github.com/devpablocristo/pymes/v3/backend/internal/worker/usecases/domain"
)

type Dispatcher interface {
	DispatchOnce(context.Context) error
}

// Dispatchers composes independently owned context workers without giving any
// of them access to another context's repository. One failure does not starve
// the remaining dispatchers in the same tick.
type Dispatchers []Dispatcher

func (dispatchers Dispatchers) DispatchOnce(ctx context.Context) error {
	var result error
	for index, dispatcher := range dispatchers {
		if ctx.Err() != nil {
			return errors.Join(result, ctx.Err())
		}
		if dispatcher == nil {
			result = errors.Join(result, fmt.Errorf("dispatcher %d is not configured", index))
			continue
		}
		if err := dispatcher.DispatchOnce(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("dispatcher %d: %w", index, err))
		}
	}
	return result
}

type MetricsReader interface {
	Collect(context.Context) (workerdomain.Metrics, error)
}

type CircuitState interface {
	CircuitOpen() bool
}

// EventTrace carries only durable W3C context; payload and tenant data stay outside it.
type EventTrace struct {
	Name        string
	TraceParent string
}

type EventSpan interface {
	End(error)
}

type EventTracer interface {
	ContinueEvent(context.Context, EventTrace) (context.Context, EventSpan)
}

type Runner struct {
	Dispatcher     Dispatcher
	Metrics        MetricsReader
	Circuits       map[string]CircuitState
	Logger         *slog.Logger
	DispatchEvery  time.Duration
	MetricsEvery   time.Duration
	MetricsTimeout time.Duration
	RunOnce        bool
}

func (r Runner) Run(ctx context.Context) error {
	if r.Dispatcher == nil || r.Metrics == nil {
		return fmt.Errorf("worker runtime dependencies are not configured")
	}
	if r.DispatchEvery <= 0 || r.MetricsEvery <= 0 {
		return fmt.Errorf("worker runtime intervals are invalid")
	}
	logger := r.Logger
	if logger == nil {
		logger = slog.Default()
	}
	metricsTimeout := r.MetricsTimeout
	if metricsTimeout <= 0 {
		metricsTimeout = 5 * time.Second
	}

	emitMetrics(ctx, logger, r.Metrics, r.Circuits, metricsTimeout)
	if r.RunOnce {
		if ctx.Err() != nil {
			return nil
		}
		if err := r.Dispatcher.DispatchOnce(ctx); err != nil {
			return fmt.Errorf("one-shot worker dispatch: %w", err)
		}
		return nil
	}

	dispatchTicker := time.NewTicker(r.DispatchEvery)
	defer dispatchTicker.Stop()
	metricsTicker := time.NewTicker(r.MetricsEvery)
	defer metricsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-dispatchTicker.C:
			if ctx.Err() != nil {
				return nil
			}
			if err := r.Dispatcher.DispatchOnce(ctx); err != nil {
				logger.ErrorContext(
					ctx,
					"worker dispatch failed",
					"code",
					"DELIVERY_FAILED",
				)
			}
		case <-metricsTicker.C:
			if ctx.Err() != nil {
				return nil
			}
			emitMetrics(
				ctx,
				logger,
				r.Metrics,
				r.Circuits,
				metricsTimeout,
			)
		}
	}
}

func emitMetrics(
	ctx context.Context,
	logger *slog.Logger,
	reader MetricsReader,
	circuits map[string]CircuitState,
	timeout time.Duration,
) {
	metricsCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	metrics, err := reader.Collect(metricsCtx)
	if err != nil {
		logger.WarnContext(
			ctx,
			"worker metrics unavailable",
			"event",
			"worker_metrics",
			"ready",
			false,
			"code",
			"METRICS_UNAVAILABLE",
		)
		return
	}
	logMetrics(ctx, logger, metrics, circuits)
}

func logMetrics(
	ctx context.Context,
	logger *slog.Logger,
	metrics workerdomain.Metrics,
	circuits map[string]CircuitState,
) {
	fiscalCircuitOpen := circuitOpen(circuits, "fiscal")
	accountingCircuitOpen := circuitOpen(circuits, "accounting")
	openCircuits := 0
	if fiscalCircuitOpen {
		openCircuits++
	}
	if accountingCircuitOpen {
		openCircuits++
	}
	logger.InfoContext(
		ctx,
		"worker metrics",
		"event",
		"worker_metrics",
		"ready",
		true,
		"outbox_pending",
		metrics.OutboxPending,
		"outbox_leased",
		metrics.OutboxLeased,
		"outbox_retrying",
		metrics.OutboxRetrying,
		"outbox_dead_letters",
		metrics.OutboxDeadLetters,
		"outbox_oldest_age_seconds",
		metrics.OutboxOldestAgeSeconds,
		"fiscal_uncertain",
		metrics.FiscalUncertain,
		"notifications_stalled",
		metrics.NotificationsStalled,
		"notifications_failed",
		metrics.NotificationsFailed,
		"accounting_applications_pending",
		metrics.ApplicationPending,
		"accounting_reversals_pending",
		metrics.ReversalPending,
		"dependency_circuits_open",
		openCircuits,
		"fiscal_circuit_open",
		fiscalCircuitOpen,
		"accounting_circuit_open",
		accountingCircuitOpen,
	)
}

func circuitOpen(
	circuits map[string]CircuitState,
	name string,
) bool {
	circuit, exists := circuits[name]
	return exists && circuit != nil && circuit.CircuitOpen()
}
