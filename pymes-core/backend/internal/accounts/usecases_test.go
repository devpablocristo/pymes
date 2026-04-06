package accounts

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	accountsdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/accounts/usecases/domain"
)

type fakeRepository struct {
	listAccountType string
	listEntityType  string
	createInput     accountsdomain.Account
}

func (f *fakeRepository) List(_ context.Context, _ uuid.UUID, accountType, entityType string, _ bool, _ int) ([]accountsdomain.Account, error) {
	f.listAccountType = accountType
	f.listEntityType = entityType
	return nil, nil
}

func (f *fakeRepository) ListMovements(_ context.Context, _, _ uuid.UUID, _ int) ([]accountsdomain.Movement, error) {
	return nil, nil
}

func (f *fakeRepository) CreateOrAdjust(_ context.Context, in accountsdomain.Account, _ float64, _, _ string) (accountsdomain.Account, error) {
	f.createInput = in
	return in, nil
}

func TestListRejectsInconsistentTypeAndEntityType(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	uc := NewUsecases(repo)

	_, err := uc.List(context.Background(), uuid.New(), "receivable", "supplier", false, 20)
	if err == nil || !strings.Contains(err.Error(), "type and entity_type are inconsistent") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCreateOrAdjustDerivesEntityTypeFromType(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	uc := NewUsecases(repo)

	_, err := uc.CreateOrAdjust(context.Background(), accountsdomain.Account{
		OrgID:      uuid.New(),
		Type:       "receivable",
		EntityID:   uuid.New(),
		EntityName: "Cliente Demo",
		Currency:   "ARS",
	}, 100, "ajuste", "tester")
	if err != nil {
		t.Fatalf("CreateOrAdjust returned error: %v", err)
	}
	if repo.createInput.EntityType != "customer" {
		t.Fatalf("expected derived entity_type customer, got %q", repo.createInput.EntityType)
	}
}
