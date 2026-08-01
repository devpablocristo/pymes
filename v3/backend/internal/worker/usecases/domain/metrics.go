// Package domain defines the worker's operational model.
package domain

type Metrics struct {
	OutboxPending          int64
	OutboxLeased           int64
	OutboxRetrying         int64
	OutboxDeadLetters      int64
	OutboxOldestAgeSeconds float64
	FiscalUncertain        int64
	NotificationsStalled   int64
	NotificationsFailed    int64
	ApplicationPending     int64
	ReversalPending        int64
}
