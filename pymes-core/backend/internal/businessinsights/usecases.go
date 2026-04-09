package businessinsights

import "context"

type ReadRepository interface {
	ListByTenant(ctx context.Context, tenantID string, limit int) ([]CandidateRecord, error)
}

type Usecases struct {
	repo ReadRepository
}

func NewUsecases(repo ReadRepository) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) List(ctx context.Context, orgID string, limit int) ([]CandidateRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return u.repo.ListByTenant(ctx, orgID, limit)
}
