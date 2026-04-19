package customers

import (
	"context"
	"testing"

	"github.com/google/uuid"

	customerdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/customers/usecases/domain"
)

type customersRepoStub struct {
	created customerdomain.Customer
	current customerdomain.Customer
	updated customerdomain.Customer
}

func (s *customersRepoStub) List(context.Context, ListParams) ([]customerdomain.Customer, int64, bool, *uuid.UUID, error) {
	return nil, 0, false, nil, nil
}
func (s *customersRepoStub) ListArchived(context.Context, uuid.UUID) ([]customerdomain.Customer, error) {
	return nil, nil
}
func (s *customersRepoStub) Create(_ context.Context, in customerdomain.Customer) (customerdomain.Customer, error) {
	s.created = in
	if in.ID == uuid.Nil {
		in.ID = uuid.New()
	}
	return in, nil
}
func (s *customersRepoStub) GetByID(context.Context, uuid.UUID, uuid.UUID) (customerdomain.Customer, error) {
	return s.current, nil
}
func (s *customersRepoStub) Update(_ context.Context, in customerdomain.Customer) (customerdomain.Customer, error) {
	s.updated = in
	return in, nil
}
func (s *customersRepoStub) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (s *customersRepoStub) Restore(context.Context, uuid.UUID, uuid.UUID) error    { return nil }
func (s *customersRepoStub) HardDelete(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (s *customersRepoStub) ListSales(context.Context, uuid.UUID, uuid.UUID) ([]customerdomain.SaleHistoryItem, error) {
	return nil, nil
}

func TestNormalizeArgentinaPhone(t *testing.T) {
	tests := map[string]string{
		"3815551234":       "+543815551234",
		"0381 555-1234":    "+543815551234",
		"+54 381 555 1234": "+543815551234",
		"+5493815551234":   "+5493815551234",
		"":                 "",
	}
	for input, want := range tests {
		if got := normalizeArgentinaPhone(input); got != want {
			t.Fatalf("normalizeArgentinaPhone(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCreateNormalizesPhone(t *testing.T) {
	repo := &customersRepoStub{}
	uc := NewUsecases(repo, nil)
	orgID := uuid.New()

	_, err := uc.Create(context.Background(), customerdomain.Customer{
		OrgID: orgID,
		Name:  "Cliente Demo",
		Type:  "person",
		Phone: "0381 555-1234",
	}, "tester")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if repo.created.Phone != "+543815551234" {
		t.Fatalf("created phone = %q, want %q", repo.created.Phone, "+543815551234")
	}
}

func TestUpdateNormalizesPhone(t *testing.T) {
	orgID := uuid.New()
	customerID := uuid.New()
	repo := &customersRepoStub{
		current: customerdomain.Customer{
			ID:    customerID,
			OrgID: orgID,
			Name:  "Cliente Demo",
			Type:  "person",
			Phone: "3815550000",
		},
	}
	uc := NewUsecases(repo, nil)
	phone := "0381 555-9999"

	_, err := uc.Update(context.Background(), orgID, customerID, UpdateInput{
		Phone: &phone,
	}, "tester")
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if repo.updated.Phone != "+543815559999" {
		t.Fatalf("updated phone = %q, want %q", repo.updated.Phone, "+543815559999")
	}
}
