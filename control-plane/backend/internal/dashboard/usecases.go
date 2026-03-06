package dashboard

import (
	"context"

	"github.com/google/uuid"

	dashboarddomain "github.com/devpablocristo/pymes/control-plane/backend/internal/dashboard/usecases/domain"
)

type RepositoryPort interface {
	Get(ctx context.Context, orgID uuid.UUID) (dashboarddomain.Dashboard, error)
}

type Usecases struct { repo RepositoryPort }

func NewUsecases(repo RepositoryPort) *Usecases { return &Usecases{repo: repo} }

func (u *Usecases) Get(ctx context.Context, orgID uuid.UUID) (dashboarddomain.Dashboard, error) {
	return u.repo.Get(ctx, orgID)
}
