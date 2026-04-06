// Package reports provides persistence for reporting queries.
package reports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	reportdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/reports/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) SalesSummary(ctx context.Context, orgID uuid.UUID, from, to time.Time) (reportdomain.SalesSummary, error) {
	type row struct {
		Total float64 `gorm:"column:total"`
		Count int64   `gorm:"column:count"`
		Avg   float64 `gorm:"column:avg"`
	}
	var agg row
	if err := r.db.WithContext(ctx).Table("sales").
		Select(`
			COALESCE(SUM(total), 0) AS total,
			COUNT(*) AS count,
			COALESCE(AVG(total), 0) AS avg
		`).
		Where("org_id = ? AND status = 'completed' AND created_at >= ? AND created_at <= ?", orgID, from, to).
		Take(&agg).Error; err != nil {
		return reportdomain.SalesSummary{}, err
	}
	return reportdomain.SalesSummary{
		TotalSales:    agg.Total,
		CountSales:    agg.Count,
		AverageTicket: agg.Avg,
	}, nil
}

func (r *Repository) SalesByProduct(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]reportdomain.SalesByProductItem, error) {
	type row struct {
		ProductID   *uuid.UUID `gorm:"column:product_id"`
		ProductName string     `gorm:"column:product_name"`
		Quantity    float64    `gorm:"column:quantity"`
		Revenue     float64    `gorm:"column:revenue"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("sale_items si").
		Select(`
			si.product_id,
			COALESCE(p.name, si.description) AS product_name,
			COALESCE(SUM(si.quantity), 0) AS quantity,
			COALESCE(SUM(si.subtotal), 0) AS revenue
		`).
		Joins("JOIN sales s ON s.id = si.sale_id").
		Joins("LEFT JOIN products p ON p.id = si.product_id").
		Where("s.org_id = ? AND s.status = 'completed' AND s.created_at >= ? AND s.created_at <= ?", orgID, from, to).
		Group("si.product_id, COALESCE(p.name, si.description)").
		Order("revenue DESC").
		Limit(100).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]reportdomain.SalesByProductItem, 0, len(rows))
	for _, row := range rows {
		item := reportdomain.SalesByProductItem{
			ProductName: row.ProductName,
			Quantity:    row.Quantity,
			Revenue:     row.Revenue,
		}
		if row.ProductID != nil {
			item.ProductID = row.ProductID.String()
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *Repository) SalesByService(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]reportdomain.SalesByServiceItem, error) {
	type row struct {
		ServiceID   *uuid.UUID `gorm:"column:service_id"`
		ServiceName string     `gorm:"column:service_name"`
		Quantity    float64    `gorm:"column:quantity"`
		Revenue     float64    `gorm:"column:revenue"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("sale_items si").
		Select(`
			si.service_id,
			COALESCE(sv.name, si.description) AS service_name,
			COALESCE(SUM(si.quantity), 0) AS quantity,
			COALESCE(SUM(si.subtotal), 0) AS revenue
		`).
		Joins("JOIN sales s ON s.id = si.sale_id").
		Joins("LEFT JOIN services sv ON sv.id = si.service_id").
		Where("s.org_id = ? AND s.status = 'completed' AND s.created_at >= ? AND s.created_at <= ? AND si.service_id IS NOT NULL", orgID, from, to).
		Group("si.service_id, COALESCE(sv.name, si.description)").
		Order("revenue DESC").
		Limit(100).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]reportdomain.SalesByServiceItem, 0, len(rows))
	for _, row := range rows {
		item := reportdomain.SalesByServiceItem{
			ServiceName: row.ServiceName,
			Quantity:    row.Quantity,
			Revenue:     row.Revenue,
		}
		if row.ServiceID != nil {
			item.ServiceID = row.ServiceID.String()
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *Repository) SalesByCustomer(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]reportdomain.SalesByCustomerItem, error) {
	type row struct {
		CustomerID   *uuid.UUID `gorm:"column:customer_id"`
		CustomerName string     `gorm:"column:customer_name"`
		Total        float64    `gorm:"column:total"`
		Count        int64      `gorm:"column:count"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).Table("sales").
		Select(`
			party_id AS customer_id,
			COALESCE(NULLIF(party_name, ''), 'Unknown') AS customer_name,
			COALESCE(SUM(total), 0) AS total,
			COUNT(*) AS count
		`).
		Where("org_id = ? AND status = 'completed' AND created_at >= ? AND created_at <= ?", orgID, from, to).
		Group("party_id, COALESCE(NULLIF(party_name, ''), 'Unknown')").
		Order("total DESC").
		Limit(100).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]reportdomain.SalesByCustomerItem, 0, len(rows))
	for _, row := range rows {
		item := reportdomain.SalesByCustomerItem{
			CustomerName: row.CustomerName,
			Total:        row.Total,
			Count:        row.Count,
		}
		if row.CustomerID != nil {
			item.CustomerID = row.CustomerID.String()
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *Repository) SalesByPayment(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]reportdomain.SalesByPaymentItem, error) {
	var out []reportdomain.SalesByPaymentItem
	if err := r.db.WithContext(ctx).Table("sales").
		Select("payment_method, COALESCE(SUM(total), 0) AS total, COUNT(*) AS count").
		Where("org_id = ? AND status = 'completed' AND created_at >= ? AND created_at <= ?", orgID, from, to).
		Group("payment_method").
		Order("total DESC").
		Scan(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) InventoryValuation(ctx context.Context, orgID uuid.UUID) ([]reportdomain.InventoryValuationItem, float64, error) {
	type row struct {
		ProductID   uuid.UUID `gorm:"column:product_id"`
		ProductName string    `gorm:"column:product_name"`
		SKU         string    `gorm:"column:sku"`
		Quantity    float64   `gorm:"column:quantity"`
		CostPrice   float64   `gorm:"column:cost_price"`
		Valuation   float64   `gorm:"column:valuation"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).Table("stock_levels sl").
		Select(`
			sl.product_id,
			p.name AS product_name,
			p.sku,
			sl.quantity,
			p.cost_price,
			(sl.quantity * p.cost_price) AS valuation
		`).
		Joins("JOIN products p ON p.id = sl.product_id").
		Where("sl.org_id = ? AND p.deleted_at IS NULL", orgID).
		Order("valuation DESC").
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	total := 0.0
	out := make([]reportdomain.InventoryValuationItem, 0, len(rows))
	for _, row := range rows {
		total += row.Valuation
		out = append(out, reportdomain.InventoryValuationItem{
			ProductID:   row.ProductID.String(),
			ProductName: row.ProductName,
			SKU:         row.SKU,
			Quantity:    row.Quantity,
			CostPrice:   row.CostPrice,
			Valuation:   row.Valuation,
		})
	}
	return out, total, nil
}

func (r *Repository) LowStock(ctx context.Context, orgID uuid.UUID) ([]reportdomain.LowStockItem, error) {
	out := make([]reportdomain.LowStockItem, 0)
	if err := r.db.WithContext(ctx).Table("stock_levels sl").
		Select("sl.product_id::text AS product_id, p.name AS product_name, p.sku, sl.quantity, sl.min_quantity").
		Joins("JOIN products p ON p.id = sl.product_id").
		Where("sl.org_id = ? AND p.deleted_at IS NULL AND sl.min_quantity > 0 AND sl.quantity <= sl.min_quantity", orgID).
		Order("sl.quantity ASC").
		Scan(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) CashflowSummary(ctx context.Context, orgID uuid.UUID, from, to time.Time) (reportdomain.CashflowSummary, error) {
	type row struct {
		Income  float64 `gorm:"column:income"`
		Expense float64 `gorm:"column:expense"`
	}
	var agg row
	if err := r.db.WithContext(ctx).Table("cash_movements").
		Select(`
			COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END), 0) AS income,
			COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0) AS expense
		`).
		Where("org_id = ? AND created_at >= ? AND created_at <= ?", orgID, from, to).
		Take(&agg).Error; err != nil {
		return reportdomain.CashflowSummary{}, err
	}
	return reportdomain.CashflowSummary{
		TotalIncome:  agg.Income,
		TotalExpense: agg.Expense,
		Balance:      agg.Income - agg.Expense,
	}, nil
}

func (r *Repository) ProfitMargin(ctx context.Context, orgID uuid.UUID, from, to time.Time) (reportdomain.ProfitMargin, error) {
	type row struct {
		Revenue float64 `gorm:"column:revenue"`
		Cost    float64 `gorm:"column:cost"`
	}
	var agg row
	if err := r.db.WithContext(ctx).Table("sale_items si").
		Select(`
			COALESCE(SUM(si.quantity * si.unit_price), 0) AS revenue,
			COALESCE(SUM(si.quantity * si.cost_price), 0) AS cost
		`).
		Joins("JOIN sales s ON s.id = si.sale_id").
		Where("s.org_id = ? AND s.status = 'completed' AND s.created_at >= ? AND s.created_at <= ?", orgID, from, to).
		Take(&agg).Error; err != nil {
		return reportdomain.ProfitMargin{}, err
	}
	gross := agg.Revenue - agg.Cost
	marginPct := 0.0
	if agg.Revenue > 0 {
		marginPct = (gross / agg.Revenue) * 100
	}
	return reportdomain.ProfitMargin{
		Revenue:     agg.Revenue,
		Cost:        agg.Cost,
		GrossProfit: gross,
		MarginPct:   marginPct,
	}, nil
}
