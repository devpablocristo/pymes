package reports

import (
	"context"
	"time"

	"github.com/google/uuid"

	reportdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/reports/usecases/domain"
)

type RepositoryPort interface {
	SalesSummary(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) (reportdomain.SalesSummary, error)
	SalesByProduct(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) ([]reportdomain.SalesByProductItem, error)
	SalesByService(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) ([]reportdomain.SalesByServiceItem, error)
	SalesByCustomer(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) ([]reportdomain.SalesByCustomerItem, error)
	SalesByPayment(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) ([]reportdomain.SalesByPaymentItem, error)
	InventoryValuation(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]reportdomain.InventoryValuationItem, float64, error)
	LowStock(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]reportdomain.LowStockItem, error)
	CashflowSummary(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) (reportdomain.CashflowSummary, error)
	ProfitMargin(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) (reportdomain.ProfitMargin, error)
}

type Usecases struct {
	repo RepositoryPort
}

func NewUsecases(repo RepositoryPort) *Usecases { return &Usecases{repo: repo} }

func (u *Usecases) SalesSummary(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) (reportdomain.SalesSummary, error) {
	return u.repo.SalesSummary(ctx, orgID, branchID, from, to)
}

func (u *Usecases) SalesByProduct(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) ([]reportdomain.SalesByProductItem, error) {
	return u.repo.SalesByProduct(ctx, orgID, branchID, from, to)
}

func (u *Usecases) SalesByService(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) ([]reportdomain.SalesByServiceItem, error) {
	return u.repo.SalesByService(ctx, orgID, branchID, from, to)
}

func (u *Usecases) SalesByCustomer(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) ([]reportdomain.SalesByCustomerItem, error) {
	return u.repo.SalesByCustomer(ctx, orgID, branchID, from, to)
}

func (u *Usecases) SalesByPayment(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) ([]reportdomain.SalesByPaymentItem, error) {
	return u.repo.SalesByPayment(ctx, orgID, branchID, from, to)
}

func (u *Usecases) InventoryValuation(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]reportdomain.InventoryValuationItem, float64, error) {
	return u.repo.InventoryValuation(ctx, orgID, branchID)
}

func (u *Usecases) LowStock(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]reportdomain.LowStockItem, error) {
	return u.repo.LowStock(ctx, orgID, branchID)
}

func (u *Usecases) CashflowSummary(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) (reportdomain.CashflowSummary, error) {
	return u.repo.CashflowSummary(ctx, orgID, branchID, from, to)
}

func (u *Usecases) ProfitMargin(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) (reportdomain.ProfitMargin, error) {
	return u.repo.ProfitMargin(ctx, orgID, branchID, from, to)
}
