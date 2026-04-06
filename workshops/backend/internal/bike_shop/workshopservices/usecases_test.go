package workshopservices

import (
	"context"
	"testing"

	"github.com/google/uuid"

	domain "github.com/devpablocristo/pymes/workshops/backend/internal/bike_shop/workshopservices/usecases/domain"
)

type fakeRepo struct {
	created domain.Service
}

func (f *fakeRepo) List(ctx context.Context, p ListParams) ([]domain.Service, int64, bool, *uuid.UUID, error) {
	_ = ctx
	_ = p
	return nil, 0, false, nil, nil
}

func (f *fakeRepo) Create(ctx context.Context, in domain.Service) (domain.Service, error) {
	_ = ctx
	f.created = in
	in.ID = uuid.New()
	return in, nil
}

func (f *fakeRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Service, error) {
	_ = ctx
	_ = orgID
	_ = id
	return domain.Service{}, nil
}

func (f *fakeRepo) Update(ctx context.Context, in domain.Service) (domain.Service, error) {
	_ = ctx
	return in, nil
}

type fakeCP struct {
	service map[string]any
}

func (f *fakeCP) GetService(ctx context.Context, orgID, serviceID string) (map[string]any, error) {
	_ = ctx
	_ = orgID
	_ = serviceID
	return f.service, nil
}

func TestCreateEnrichesLinkedServiceDefaults(t *testing.T) {
	repo := &fakeRepo{}
	cp := &fakeCP{service: map[string]any{
		"description": "Diagnóstico de e-bike",
		"sale_price":  15000.0,
		"tax_rate":    10.5,
	}}
	uc := NewUsecases(repo, nil, cp)
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	serviceID := uuid.MustParse("00000000-0000-0000-0000-000000000010")

	out, err := uc.Create(context.Background(), domain.Service{
		OrgID:           orgID,
		Code:            "EBIKE",
		Name:            "Diagnóstico e-bike",
		LinkedServiceID: &serviceID,
	}, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if out.Description != "Diagnóstico de e-bike" {
		t.Fatalf("Create().Description = %q, want enriched description", out.Description)
	}
	if out.BasePrice != 15000 {
		t.Fatalf("Create().BasePrice = %v, want 15000", out.BasePrice)
	}
	if out.TaxRate != 10.5 {
		t.Fatalf("Create().TaxRate = %v, want 10.5", out.TaxRate)
	}
}
