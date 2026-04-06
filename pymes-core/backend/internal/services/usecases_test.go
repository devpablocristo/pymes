package services

import (
	"context"
	"testing"

	"github.com/google/uuid"

	servicedomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/services/usecases/domain"
)

type captureServiceRepo struct {
	created servicedomain.Service
}

func (r *captureServiceRepo) List(context.Context, ListParams) ([]servicedomain.Service, int64, bool, *uuid.UUID, error) {
	return nil, 0, false, nil, nil
}
func (r *captureServiceRepo) Create(_ context.Context, in servicedomain.Service) (servicedomain.Service, error) {
	r.created = in
	return in, nil
}
func (r *captureServiceRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (servicedomain.Service, error) {
	return servicedomain.Service{}, nil
}
func (r *captureServiceRepo) Update(context.Context, servicedomain.Service) (servicedomain.Service, error) {
	return servicedomain.Service{}, nil
}
func (r *captureServiceRepo) Archive(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r *captureServiceRepo) Restore(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r *captureServiceRepo) Delete(context.Context, uuid.UUID, uuid.UUID) error  { return nil }

func TestCreateDefaultsCurrency(t *testing.T) {
	t.Parallel()

	repo := &captureServiceRepo{}
	uc := NewUsecases(repo, nil)

	out, err := uc.Create(context.Background(), servicedomain.Service{
		OrgID: uuid.New(),
		Name:  "Servicio demo",
	}, "tester")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.Currency != "ARS" {
		t.Fatalf("expected currency ARS, got %q", out.Currency)
	}
	if repo.created.Currency != "ARS" {
		t.Fatalf("expected repo to receive currency ARS, got %q", repo.created.Currency)
	}
}
