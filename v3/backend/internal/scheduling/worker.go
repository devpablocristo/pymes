// architecture:adapter worker
package scheduling

import (
	"context"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	workerhelpers "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/worker/helpers"
	workermodels "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/worker/models"
)

// MaintenanceUsecases is the worker-owned input port. The worker knows
// nothing about PostgreSQL, Platform or notification delivery.
type MaintenanceUsecases interface {
	RunMaintenance(context.Context, int) (domain.MaintenanceResult, error)
}

type Worker struct {
	usecases MaintenanceUsecases
	limit    int
}

func NewWorker(usecases MaintenanceUsecases, limit int) *Worker {
	return &Worker{usecases: usecases, limit: workerhelpers.NormalizeLimit(limit)}
}

func (w *Worker) DispatchOnce(ctx context.Context) error {
	_, err := w.RunOnce(ctx)
	return err
}

func (w *Worker) RunOnce(ctx context.Context) (workermodels.RunResult, error) {
	result, err := w.usecases.RunMaintenance(ctx, w.limit)
	if err != nil {
		return workermodels.RunResult{}, err
	}
	return workermodels.RunResult{
		ExpiredHolds:   result.ExpiredHolds,
		ReminderEvents: result.ReminderEvents,
		WaitlistOffers: result.WaitlistOffers,
	}, nil
}
