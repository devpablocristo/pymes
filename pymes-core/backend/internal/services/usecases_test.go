package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	servicedomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/services/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type captureServiceRepo struct {
	created    servicedomain.Service
	existing   *servicedomain.Service
	getByIDErr error
}

func (r *captureServiceRepo) List(context.Context, ListParams) ([]servicedomain.Service, int64, bool, *uuid.UUID, error) {
	return nil, 0, false, nil, nil
}
func (r *captureServiceRepo) Create(_ context.Context, in servicedomain.Service) (servicedomain.Service, error) {
	r.created = in
	return in, nil
}
func (r *captureServiceRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (servicedomain.Service, error) {
	if r.getByIDErr != nil {
		return servicedomain.Service{}, r.getByIDErr
	}
	if r.existing != nil {
		return *r.existing, nil
	}
	return servicedomain.Service{}, nil
}
func (r *captureServiceRepo) Update(_ context.Context, in servicedomain.Service) (servicedomain.Service, error) {
	return in, nil
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

// TestUpdateMapsNotFoundSentinelToHTTPNotFound: update sobre servicio
// inexistente debe devolver httperrors.ErrNotFound (404), no caer como 500.
func TestUpdateMapsNotFoundSentinelToHTTPNotFound(t *testing.T) {
	t.Parallel()

	repo := &captureServiceRepo{getByIDErr: ErrNotFound}
	uc := NewUsecases(repo, nil)

	_, err := uc.Update(context.Background(), uuid.New(), uuid.New(), UpdateInput{}, "tester")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Fatalf("expected httperrors.ErrNotFound, got %v", err)
	}
}

// TestUpdateRejectsArchivedServiceWithConflict: update sobre archivado
// debe devolver 409 Conflict, no 404 ni 500.
func TestUpdateRejectsArchivedServiceWithConflict(t *testing.T) {
	t.Parallel()

	archivedAt := time.Now().UTC()
	orgID := uuid.New()
	svcID := uuid.New()
	repo := &captureServiceRepo{existing: &servicedomain.Service{
		ID:        svcID,
		OrgID:     orgID,
		Name:      "Servicio archivado",
		DeletedAt: &archivedAt,
	}}
	uc := NewUsecases(repo, nil)

	_, err := uc.Update(context.Background(), orgID, svcID, UpdateInput{}, "tester")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, httperrors.ErrConflict) {
		t.Fatalf("expected httperrors.ErrConflict, got %v", err)
	}
}
