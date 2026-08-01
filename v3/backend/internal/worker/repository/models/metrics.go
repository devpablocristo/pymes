// Package models contains aggregation records owned by the worker repository.
package models

type MetricsAccumulator struct {
	Metrics Metrics
}

type Metrics struct {
	OutboxPending          int64
	OutboxLeased           int64
	OutboxRetrying         int64
	OutboxDeadLetters      int64
	OutboxOldestAgeSeconds float64
	FiscalUncertain        int64
	ApplicationPending     int64
	ReversalPending        int64
}
