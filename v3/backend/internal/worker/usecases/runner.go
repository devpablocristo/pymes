// Package usecases contains the worker's application orchestration.
package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	workerdomain "github.com/devpablocristo/pymes/v3/backend/internal/worker/domain"
	workerports "github.com/devpablocristo/pymes/v3/backend/internal/worker/ports"
)

type Runner struct {
	Dispatcher     workerports.Dispatcher
	Metrics        workerports.MetricsReader
	Circuits       map[string]workerports.CircuitState
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
	reader workerports.MetricsReader,
	circuits map[string]workerports.CircuitState,
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
	circuits map[string]workerports.CircuitState,
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
	circuits map[string]workerports.CircuitState,
	name string,
) bool {
	circuit, exists := circuits[name]
	return exists && circuit != nil && circuit.CircuitOpen()
}
