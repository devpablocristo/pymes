package helpers

import (
	"testing"

	"github.com/devpablocristo/pymes/v3/backend/internal/worker/repository/models"
)

func TestAddMetricsKeepsOldestAgeMaximum(t *testing.T) {
	target := models.MetricsAccumulator{}
	AddMetrics(&target, models.Metrics{OutboxOldestAgeSeconds: 9})
	AddMetrics(&target, models.Metrics{OutboxOldestAgeSeconds: 3})
	if target.Metrics.OutboxOldestAgeSeconds != 9 {
		t.Fatalf("got %v", target.Metrics.OutboxOldestAgeSeconds)
	}
}
