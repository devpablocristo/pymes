package sales

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/inventory"
	saledomain "github.com/devpablocristo/pymes/control-plane/backend/internal/sales/usecases/domain"
)

type mockRepo struct {
	getTenantSettingsFn func(ctx context.Context, orgID uuid.UUID) (string, float64, string, error)
	getProductFn        func(ctx context.Context, orgID, productID uuid.UUID) (ProductSnapshot, error)
	createFn            func(ctx context.Context, in CreateInput) (saledomain.Sale, error)
	getByIDFn           func(ctx context.Context, orgID, saleID uuid.UUID) (saledomain.Sale, error)
	voidFn              func(ctx context.Context, orgID, saleID uuid.UUID) (saledomain.Sale, error)
}

func (m *mockRepo) List(ctx context.Context, p ListParams) ([]saledomain.Sale, int64, bool, *uuid.UUID, error) {
	return nil, 0, false, nil, nil
}
func (m *mockRepo) Create(ctx context.Context, in CreateInput) (saledomain.Sale, error) {
	return m.createFn(ctx, in)
}
func (m *mockRepo) GetByID(ctx context.Context, orgID, saleID uuid.UUID) (saledomain.Sale, error) {
	return m.getByIDFn(ctx, orgID, saleID)
}
func (m *mockRepo) Void(ctx context.Context, orgID, saleID uuid.UUID) (saledomain.Sale, error) {
	return m.voidFn(ctx, orgID, saleID)
}
func (m *mockRepo) GetTenantSettings(ctx context.Context, orgID uuid.UUID) (string, float64, string, error) {
	return m.getTenantSettingsFn(ctx, orgID)
}
func (m *mockRepo) GetProductSnapshot(ctx context.Context, orgID, productID uuid.UUID) (ProductSnapshot, error) {
	return m.getProductFn(ctx, orgID, productID)
}

type mockInventory struct {
	applyCalled   int
	reverseCalled int
	lastApply     []inventory.SaleItemStock
	lastReverse   []inventory.SaleItemStock
}

func (m *mockInventory) ApplySaleItems(ctx context.Context, orgID, saleID uuid.UUID, actor string, items []inventory.SaleItemStock) error {
	m.applyCalled++
	m.lastApply = items
	return nil
}
func (m *mockInventory) ReverseSaleItems(ctx context.Context, orgID, saleID uuid.UUID, actor string, items []inventory.SaleItemStock) error {
	m.reverseCalled++
	m.lastReverse = items
	return nil
}

type mockCashflow struct {
	incomeCalled int
	voidCalled   int
	lastIncome   float64
	lastVoid     float64
}

func (m *mockCashflow) RecordSaleIncome(ctx context.Context, orgID, saleID uuid.UUID, amount float64, currency, paymentMethod, actor string) error {
	m.incomeCalled++
	m.lastIncome = amount
	return nil
}
func (m *mockCashflow) RecordSaleVoidExpense(ctx context.Context, orgID, saleID uuid.UUID, amount float64, currency, paymentMethod, actor string) error {
	m.voidCalled++
	m.lastVoid = amount
	return nil
}

type mockAudit struct{ calls int }

func (m *mockAudit) Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any) {
	m.calls++
}

func TestCreateSale_AppliesStockAndCashflow(t *testing.T) {
	orgID := uuid.New()
	productID := uuid.New()
	saleID := uuid.New()

	repo := &mockRepo{
		getTenantSettingsFn: func(ctx context.Context, orgID uuid.UUID) (string, float64, string, error) {
			return "ARS", 21.0, "VTA", nil
		},
		getProductFn: func(ctx context.Context, orgID, productID uuid.UUID) (ProductSnapshot, error) {
			return ProductSnapshot{
				ID:         productID,
				Name:       "Prod A",
				Price:      100,
				CostPrice:  60,
				TaxRate:    nil,
				TrackStock: true,
			}, nil
		},
		createFn: func(ctx context.Context, in CreateInput) (saledomain.Sale, error) {
			if in.Subtotal != 200 {
				t.Fatalf("expected subtotal 200, got %v", in.Subtotal)
			}
			if in.Total != 242 {
				t.Fatalf("expected total 242, got %v", in.Total)
			}
			return saledomain.Sale{
				ID:            saleID,
				OrgID:         in.OrgID,
				Status:        "completed",
				PaymentMethod: "cash",
				Total:         in.Total,
				Currency:      in.Currency,
				Items: []saledomain.SaleItem{
					{
						ProductID: &productID,
						Quantity:  2,
					},
				},
			}, nil
		},
		getByIDFn: func(ctx context.Context, orgID, saleID uuid.UUID) (saledomain.Sale, error) {
			return saledomain.Sale{}, nil
		},
		voidFn: func(ctx context.Context, orgID, saleID uuid.UUID) (saledomain.Sale, error) {
			return saledomain.Sale{}, nil
		},
	}

	inv := &mockInventory{}
	cash := &mockCashflow{}
	audit := &mockAudit{}
	uc := NewUsecases(repo, inv, cash, audit)

	_, err := uc.Create(context.Background(), CreateSaleInput{
		OrgID:         orgID,
		PaymentMethod: "cash",
		Items: []CreateSaleItemInput{
			{
				ProductID: &productID,
				Quantity:  2,
				UnitPrice: 100,
			},
		},
		CreatedBy: "tester",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if inv.applyCalled != 1 {
		t.Fatalf("expected inventory apply once, got %d", inv.applyCalled)
	}
	if cash.incomeCalled != 1 {
		t.Fatalf("expected cashflow income once, got %d", cash.incomeCalled)
	}
	if cash.lastIncome != 242 {
		t.Fatalf("expected income amount 242, got %v", cash.lastIncome)
	}
	if audit.calls == 0 {
		t.Fatalf("expected audit log call")
	}
}

func TestVoidSale_ReversesStockAndCashflow(t *testing.T) {
	orgID := uuid.New()
	saleID := uuid.New()
	productID := uuid.New()
	getByIDCalls := 0

	repo := &mockRepo{
		getTenantSettingsFn: func(ctx context.Context, orgID uuid.UUID) (string, float64, string, error) {
			return "ARS", 21, "VTA", nil
		},
		getProductFn: func(ctx context.Context, orgID, productID uuid.UUID) (ProductSnapshot, error) {
			return ProductSnapshot{}, nil
		},
		createFn: func(ctx context.Context, in CreateInput) (saledomain.Sale, error) {
			return saledomain.Sale{}, nil
		},
		getByIDFn: func(ctx context.Context, orgID, inSaleID uuid.UUID) (saledomain.Sale, error) {
			getByIDCalls++
			return saledomain.Sale{
				ID:            inSaleID,
				OrgID:         orgID,
				Status:        "completed",
				PaymentMethod: "transfer",
				Total:         500,
				Currency:      "ARS",
				Items: []saledomain.SaleItem{
					{ProductID: &productID, Quantity: 3},
				},
			}, nil
		},
		voidFn: func(ctx context.Context, orgID, saleID uuid.UUID) (saledomain.Sale, error) {
			return saledomain.Sale{
				ID:            saleID,
				OrgID:         orgID,
				Status:        "voided",
				PaymentMethod: "transfer",
				Total:         500,
				Currency:      "ARS",
			}, nil
		},
	}
	inv := &mockInventory{}
	cash := &mockCashflow{}
	audit := &mockAudit{}
	uc := NewUsecases(repo, inv, cash, audit)

	out, err := uc.Void(context.Background(), orgID, saleID, "tester")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.Status != "voided" {
		t.Fatalf("expected voided status, got %s", out.Status)
	}
	if getByIDCalls == 0 {
		t.Fatalf("expected getByID called")
	}
	if inv.reverseCalled != 1 {
		t.Fatalf("expected inventory reverse once, got %d", inv.reverseCalled)
	}
	if cash.voidCalled != 1 {
		t.Fatalf("expected cashflow void once, got %d", cash.voidCalled)
	}
	if cash.lastVoid != 500 {
		t.Fatalf("expected void amount 500, got %v", cash.lastVoid)
	}
	if audit.calls == 0 {
		t.Fatalf("expected audit log call")
	}
}
