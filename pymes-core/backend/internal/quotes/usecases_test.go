package quotes

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	quotedomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/quotes/usecases/domain"
	sales "github.com/devpablocristo/pymes/pymes-core/backend/internal/sales"
	salesdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/sales/usecases/domain"
)

type mockQuoteRepo struct {
	getTenantSettingsFn func(ctx context.Context, orgID uuid.UUID) (string, float64, string, error)
	getProductFn        func(ctx context.Context, orgID, productID uuid.UUID) (ProductSnapshot, error)
	getServiceFn        func(ctx context.Context, orgID, serviceID uuid.UUID) (ServiceSnapshot, error)
	listArchivedFn      func(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]quotedomain.Quote, error)
	createFn            func(ctx context.Context, in CreateInput) (quotedomain.Quote, error)
	getByIDFn           func(ctx context.Context, orgID, quoteID uuid.UUID) (quotedomain.Quote, error)
	setStatusFn         func(ctx context.Context, orgID, quoteID uuid.UUID, status string) (quotedomain.Quote, error)
}

func (m *mockQuoteRepo) List(ctx context.Context, p ListParams) ([]quotedomain.Quote, int64, bool, *uuid.UUID, error) {
	return nil, 0, false, nil, nil
}
func (m *mockQuoteRepo) ListArchived(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]quotedomain.Quote, error) {
	if m.listArchivedFn == nil {
		return nil, nil
	}
	return m.listArchivedFn(ctx, orgID, branchID)
}
func (m *mockQuoteRepo) Create(ctx context.Context, in CreateInput) (quotedomain.Quote, error) {
	return m.createFn(ctx, in)
}
func (m *mockQuoteRepo) GetByID(ctx context.Context, orgID, quoteID uuid.UUID) (quotedomain.Quote, error) {
	return m.getByIDFn(ctx, orgID, quoteID)
}
func (m *mockQuoteRepo) UpdateDraft(ctx context.Context, in UpdateInput) (quotedomain.Quote, error) {
	return quotedomain.Quote{}, nil
}

func (m *mockQuoteRepo) PatchAnnotations(context.Context, uuid.UUID, uuid.UUID, QuotePatchFields) (quotedomain.Quote, error) {
	return quotedomain.Quote{}, nil
}

func (m *mockQuoteRepo) DeleteDraft(ctx context.Context, orgID, quoteID uuid.UUID) error { return nil }
func (m *mockQuoteRepo) Archive(ctx context.Context, orgID, quoteID uuid.UUID) error     { return nil }
func (m *mockQuoteRepo) Restore(ctx context.Context, orgID, quoteID uuid.UUID) error     { return nil }
func (m *mockQuoteRepo) HardDelete(ctx context.Context, orgID, quoteID uuid.UUID) error  { return nil }
func (m *mockQuoteRepo) SetStatus(ctx context.Context, orgID, quoteID uuid.UUID, status string) (quotedomain.Quote, error) {
	if m.setStatusFn == nil {
		return quotedomain.Quote{}, nil
	}
	return m.setStatusFn(ctx, orgID, quoteID, status)
}
func (m *mockQuoteRepo) GetTenantSettings(ctx context.Context, orgID uuid.UUID) (string, float64, string, error) {
	return m.getTenantSettingsFn(ctx, orgID)
}
func (m *mockQuoteRepo) GetProductSnapshot(ctx context.Context, orgID, productID uuid.UUID) (ProductSnapshot, error) {
	return m.getProductFn(ctx, orgID, productID)
}
func (m *mockQuoteRepo) GetServiceSnapshot(ctx context.Context, orgID, serviceID uuid.UUID) (ServiceSnapshot, error) {
	if m.getServiceFn == nil {
		return ServiceSnapshot{}, nil
	}
	return m.getServiceFn(ctx, orgID, serviceID)
}

type mockQuoteSales struct {
	createFn func(ctx context.Context, in sales.CreateSaleInput) (salesdomain.Sale, error)
}

func (m *mockQuoteSales) Create(ctx context.Context, in sales.CreateSaleInput) (salesdomain.Sale, error) {
	return m.createFn(ctx, in)
}

type mockQuoteAudit struct{ calls int }

func (m *mockQuoteAudit) Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any) {
	m.calls++
}

func TestCreateQuote_PersistsSelectedBranch(t *testing.T) {
	orgID := uuid.New()
	branchID := uuid.New()
	productID := uuid.New()
	quoteID := uuid.New()

	repo := &mockQuoteRepo{
		getTenantSettingsFn: func(ctx context.Context, orgID uuid.UUID) (string, float64, string, error) {
			return "ARS", 21.0, "PRE", nil
		},
		getProductFn: func(ctx context.Context, orgID, productID uuid.UUID) (ProductSnapshot, error) {
			return ProductSnapshot{
				ID:    productID,
				Name:  "Producto A",
				Price: 100,
			}, nil
		},
		createFn: func(ctx context.Context, in CreateInput) (quotedomain.Quote, error) {
			if in.BranchID == nil || *in.BranchID != branchID {
				t.Fatalf("expected branch_id %s, got %#v", branchID, in.BranchID)
			}
			if in.Total != 121 {
				t.Fatalf("expected total 121, got %v", in.Total)
			}
			return quotedomain.Quote{
				ID:        quoteID,
				OrgID:     in.OrgID,
				BranchID:  in.BranchID,
				Number:    "PRE-00001",
				Status:    "draft",
				Subtotal:  in.Subtotal,
				TaxTotal:  in.TaxTotal,
				Total:     in.Total,
				Currency:  in.Currency,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}, nil
		},
		getByIDFn: func(ctx context.Context, orgID, quoteID uuid.UUID) (quotedomain.Quote, error) {
			return quotedomain.Quote{}, nil
		},
	}

	audit := &mockQuoteAudit{}
	uc := NewUsecases(repo, nil, audit)

	out, err := uc.Create(context.Background(), CreateQuoteInput{
		OrgID:     orgID,
		BranchID:  &branchID,
		CreatedBy: "tester",
		Items: []QuoteItemInput{
			{
				ProductID: &productID,
				Quantity:  1,
				UnitPrice: 100,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.BranchID == nil || *out.BranchID != branchID {
		t.Fatalf("expected output branch_id %s, got %#v", branchID, out.BranchID)
	}
	if audit.calls == 0 {
		t.Fatalf("expected audit call")
	}
}

func TestQuoteToSale_PropagatesBranchToSales(t *testing.T) {
	orgID := uuid.New()
	branchID := uuid.New()
	quoteID := uuid.New()
	productID := uuid.New()
	saleID := uuid.New()
	statusUpdated := false

	repo := &mockQuoteRepo{
		getTenantSettingsFn: func(ctx context.Context, orgID uuid.UUID) (string, float64, string, error) {
			return "ARS", 21.0, "PRE", nil
		},
		getProductFn: func(ctx context.Context, orgID, productID uuid.UUID) (ProductSnapshot, error) {
			return ProductSnapshot{}, nil
		},
		createFn: func(ctx context.Context, in CreateInput) (quotedomain.Quote, error) {
			return quotedomain.Quote{}, nil
		},
		getByIDFn: func(ctx context.Context, orgID, quoteID uuid.UUID) (quotedomain.Quote, error) {
			taxRate := 21.0
			return quotedomain.Quote{
				ID:           quoteID,
				OrgID:        orgID,
				BranchID:     &branchID,
				CustomerName: "Cliente Demo",
				Status:       "sent",
				Items: []quotedomain.QuoteItem{
					{
						ProductID:   &productID,
						Description: "Producto A",
						Quantity:    2,
						UnitPrice:   100,
						TaxRate:     taxRate,
						SortOrder:   0,
					},
				},
			}, nil
		},
		setStatusFn: func(ctx context.Context, orgID, quoteID uuid.UUID, status string) (quotedomain.Quote, error) {
			if status != "accepted" {
				t.Fatalf("expected accepted status, got %s", status)
			}
			statusUpdated = true
			return quotedomain.Quote{ID: quoteID, OrgID: orgID, Status: status}, nil
		},
	}

	salesUC := &mockQuoteSales{
		createFn: func(ctx context.Context, in sales.CreateSaleInput) (salesdomain.Sale, error) {
			if in.BranchID == nil || *in.BranchID != branchID {
				t.Fatalf("expected branch_id %s, got %#v", branchID, in.BranchID)
			}
			if in.QuoteID == nil || *in.QuoteID != quoteID {
				t.Fatalf("expected quote_id %s, got %#v", quoteID, in.QuoteID)
			}
			return salesdomain.Sale{
				ID:       saleID,
				OrgID:    orgID,
				BranchID: in.BranchID,
				Number:   "VTA-00001",
				Status:   "completed",
			}, nil
		},
	}

	uc := NewUsecases(repo, salesUC, &mockQuoteAudit{})

	out, err := uc.ToSale(context.Background(), orgID, quoteID, "cash", "ok", "tester")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.BranchID == nil || *out.BranchID != branchID {
		t.Fatalf("expected sale branch_id %s, got %#v", branchID, out.BranchID)
	}
	if !statusUpdated {
		t.Fatalf("expected quote status update after sale conversion")
	}
}
