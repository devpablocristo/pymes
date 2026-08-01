// Package helpers contains row aggregation for worker operational metrics.
package helpers

import (
	"github.com/devpablocristo/pymes/v3/backend/internal/worker/repository/models"
)

func AddMetrics(target *models.MetricsAccumulator, current models.Metrics) {
	target.Metrics.OutboxPending += current.OutboxPending
	target.Metrics.OutboxLeased += current.OutboxLeased
	target.Metrics.OutboxRetrying += current.OutboxRetrying
	target.Metrics.OutboxDeadLetters += current.OutboxDeadLetters
	if current.OutboxOldestAgeSeconds > target.Metrics.OutboxOldestAgeSeconds {
		target.Metrics.OutboxOldestAgeSeconds = current.OutboxOldestAgeSeconds
	}
	target.Metrics.FiscalUncertain += current.FiscalUncertain
	target.Metrics.NotificationsStalled += current.NotificationsStalled
	target.Metrics.NotificationsFailed += current.NotificationsFailed
	target.Metrics.ApplicationPending += current.ApplicationPending
	target.Metrics.ReversalPending += current.ReversalPending
}
