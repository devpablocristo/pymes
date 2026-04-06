package salonservices

import (
	"context"
	"testing"

	"github.com/google/uuid"

	domain "github.com/devpablocristo/pymes/beauty/backend/internal/salon/salonservices/usecases/domain"
)

type fakeRepo struct {
	created domain.SalonService
}

func (f *fakeRepo) List(ctx context.Context, p ListParams) ([]domain.SalonService, int64, bool, *uuid.UUID, error) {
	_ = ctx
	_ = p
	return nil, 0, false, nil, nil
}

func (f *fakeRepo) Create(ctx context.Context, in domain.SalonService) (domain.SalonService, error) {
	_ = ctx
	f.created = in
	in.ID = uuid.New()
	return in, nil
}

func (f *fakeRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.SalonService, error) {
	_ = ctx
	_ = orgID
	_ = id
	return domain.SalonService{}, nil
}

func (f *fakeRepo) Update(ctx context.Context, in domain.SalonService) (domain.SalonService, error) {
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
		"description": "Coloración con lavado",
		"sale_price":  32000.0,
		"tax_rate":    10.5,
	}}
	uc := NewUsecases(repo, nil, cp)
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	serviceID := uuid.MustParse("00000000-0000-0000-0000-000000000010")

	out, err := uc.Create(context.Background(), domain.SalonService{
		OrgID:           orgID,
		Code:            "COLOR",
		Name:            "Coloración",
		LinkedServiceID: &serviceID,
	}, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if out.Description != "Coloración con lavado" {
		t.Fatalf("Create().Description = %q, want enriched description", out.Description)
	}
	if out.BasePrice != 32000 {
		t.Fatalf("Create().BasePrice = %v, want 32000", out.BasePrice)
	}
	if out.TaxRate != 10.5 {
		t.Fatalf("Create().TaxRate = %v, want 10.5", out.TaxRate)
	}
}
