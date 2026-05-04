package purchases

import (
	"context"
	"testing"

	"github.com/google/uuid"

	purchasesdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/purchases/usecases/domain"
)

type mockPurchasesRepo struct {
	createFn       func(ctx context.Context, in CreateInput) (purchasesdomain.Purchase, error)
	getByIDFn      func(ctx context.Context, orgID, id uuid.UUID) (purchasesdomain.Purchase, error)
	updateStatusFn func(ctx context.Context, in UpdateStatusInput) (purchasesdomain.Purchase, error)
}

func (m *mockPurchasesRepo) List(context.Context, uuid.UUID, *uuid.UUID, string, int) ([]purchasesdomain.Purchase, error) {
	return nil, nil
}
func (m *mockPurchasesRepo) Create(ctx context.Context, in CreateInput) (purchasesdomain.Purchase, error) {
	if m.createFn == nil {
		return purchasesdomain.Purchase{}, nil
	}
	return m.createFn(ctx, in)
}
func (m *mockPurchasesRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (purchasesdomain.Purchase, error) {
	return m.getByIDFn(ctx, orgID, id)
}
func (m *mockPurchasesRepo) Update(context.Context, UpdateInput) (purchasesdomain.Purchase, error) {
	return purchasesdomain.Purchase{}, nil
}

func (m *mockPurchasesRepo) PatchAnnotations(context.Context, uuid.UUID, uuid.UUID, PurchasePatchFields) (purchasesdomain.Purchase, error) {
	return purchasesdomain.Purchase{}, nil
}
func (m *mockPurchasesRepo) UpdateStatus(ctx context.Context, in UpdateStatusInput) (purchasesdomain.Purchase, error) {
	return m.updateStatusFn(ctx, in)
}
func (m *mockPurchasesRepo) GetSupplierName(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "", nil
}
func (m *mockPurchasesRepo) GetCurrency(context.Context, uuid.UUID) string { return "ARS" }
func (m *mockPurchasesRepo) GetTaxRate(context.Context, uuid.UUID) float64 { return 21 }

type mockPurchasesAudit struct{ calls int }

func (m *mockPurchasesAudit) Log(context.Context, string, string, string, string, string, map[string]any) {
	m.calls++
}

func TestUpdateStatus_AllowsAnyConfiguredTransition(t *testing.T) {
	orgID := uuid.New()
	purchaseID := uuid.New()
	repo := &mockPurchasesRepo{
		getByIDFn: func(ctx context.Context, orgID, id uuid.UUID) (purchasesdomain.Purchase, error) {
			return purchasesdomain.Purchase{
				ID:           id,
				OrgID:        orgID,
				Number:       "CPA-00001",
				Status:       "received",
				SupplierID:   &orgID,
				SupplierName: "Proveedor",
			}, nil
		},
		updateStatusFn: func(ctx context.Context, in UpdateStatusInput) (purchasesdomain.Purchase, error) {
			if in.Status != "voided" {
				t.Fatalf("expected status voided, got %q", in.Status)
			}
			return purchasesdomain.Purchase{
				ID:           in.ID,
				OrgID:        in.OrgID,
				Number:       "CPA-00001",
				Status:       in.Status,
				SupplierID:   &orgID,
				SupplierName: "Proveedor",
			}, nil
		},
	}
	audit := &mockPurchasesAudit{}
	uc := NewUsecases(repo, audit)

	out, err := uc.UpdateStatus(context.Background(), UpdateStatusInput{
		ID:     purchaseID,
		OrgID:  orgID,
		Status: "voided",
	}, "tester")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.Status != "voided" {
		t.Fatalf("expected voided status, got %q", out.Status)
	}
	if audit.calls == 0 {
		t.Fatalf("expected audit log call")
	}
}

func TestUpdateStatus_RejectsInvalidStatus(t *testing.T) {
	orgID := uuid.New()
	purchaseID := uuid.New()
	repo := &mockPurchasesRepo{
		getByIDFn: func(ctx context.Context, orgID, id uuid.UUID) (purchasesdomain.Purchase, error) {
			return purchasesdomain.Purchase{
				ID:     id,
				OrgID:  orgID,
				Number: "CPA-00002",
				Status: "received",
			}, nil
		},
		updateStatusFn: func(ctx context.Context, in UpdateStatusInput) (purchasesdomain.Purchase, error) {
			t.Fatalf("updateStatus should not be called for invalid status")
			return purchasesdomain.Purchase{}, nil
		},
	}
	uc := NewUsecases(repo, nil)

	_, err := uc.UpdateStatus(context.Background(), UpdateStatusInput{
		ID:     purchaseID,
		OrgID:  orgID,
		Status: "invalid",
	}, "tester")
	if err == nil {
		t.Fatalf("expected business rule error")
	}
}

func TestCreate_PreservesSelectedBranch(t *testing.T) {
	orgID := uuid.New()
	branchID := uuid.New()
	repo := &mockPurchasesRepo{
		createFn: func(ctx context.Context, in CreateInput) (purchasesdomain.Purchase, error) {
			if in.BranchID == nil || *in.BranchID != branchID {
				t.Fatalf("expected branch_id %s, got %#v", branchID, in.BranchID)
			}
			return purchasesdomain.Purchase{
				ID:           uuid.New(),
				OrgID:        in.OrgID,
				BranchID:     in.BranchID,
				Number:       "CPA-00003",
				Status:       in.Status,
				SupplierName: in.SupplierName,
			}, nil
		},
	}

	uc := NewUsecases(repo, nil)
	out, err := uc.Create(context.Background(), CreateInput{
		OrgID:        orgID,
		BranchID:     &branchID,
		SupplierName: "Proveedor Demo",
		Items: []purchasesdomain.PurchaseItem{
			{Description: "Insumo", Quantity: 1, UnitCost: 100},
		},
		CreatedBy: "tester",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.BranchID == nil || *out.BranchID != branchID {
		t.Fatalf("expected output branch_id %s, got %#v", branchID, out.BranchID)
	}
}
