// Package dashboard provides persistence for dashboard data endpoints.
package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	dashboarddomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/dashboard/usecases/domain"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func normalizeBranchID(branchID *uuid.UUID) *uuid.UUID {
	if branchID == nil || *branchID == uuid.Nil {
		return nil
	}
	return branchID
}

func applyEntityBranchFilter(q *gorm.DB, alias string, branchID *uuid.UUID) *gorm.DB {
	normalizedBranchID := normalizeBranchID(branchID)
	if normalizedBranchID == nil {
		return q
	}
	return q.Where("("+alias+".branch_id = ? OR "+alias+".branch_id IS NULL)", *normalizedBranchID)
}

func applyCashflowBranchFilter(q *gorm.DB, alias string, branchID *uuid.UUID) *gorm.DB {
	normalizedBranchID := normalizeBranchID(branchID)
	if normalizedBranchID == nil {
		return q
	}
	expr := `COALESCE(` + alias + `.branch_id,
		CASE
			WHEN ` + alias + `.reference_type = 'sale' THEN (
				SELECT s.branch_id FROM sales s
				WHERE s.org_id = ` + alias + `.org_id AND s.id = ` + alias + `.reference_id
			)
			WHEN ` + alias + `.reference_type = 'return' THEN (
				SELECT s.branch_id
				FROM returns r
				JOIN sales s ON s.id = r.sale_id AND s.org_id = r.org_id
				WHERE r.org_id = ` + alias + `.org_id AND r.id = ` + alias + `.reference_id
			)
			ELSE NULL
		END
	)`
	return q.Where("("+expr+" = ? OR "+expr+" IS NULL)", *normalizedBranchID)
}

func (r *Repository) ListWidgets(ctx context.Context) ([]dashboarddomain.WidgetDefinition, error) {
	_ = ctx
	catalog := fixedDashboardWidgets()
	out := make([]dashboarddomain.WidgetDefinition, 0, len(catalog))
	out = append(out, catalog...)
	return out, nil
}

func (r *Repository) LoadSalesSummary(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) (dashboarddomain.SalesSummaryData, error) {
	period := currentPeriodLabel()
	var out dashboarddomain.SalesSummaryData
	out.Period = period
	q := r.db.WithContext(ctx).Table("sales s")
	q = applyEntityBranchFilter(q, "s", branchID)
	result := q.Select(`
			COALESCE(SUM(s.total), 0) AS total_sales,
			COUNT(*) AS count_sales,
			COALESCE(AVG(s.total), 0) AS average_ticket
		`).
		Where("s.org_id = ? AND s.status = 'completed' AND s.created_at >= ?", orgID, currentMonthStart()).
		Scan(&out)
	return out, result.Error
}

func (r *Repository) LoadCashflowSummary(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) (dashboarddomain.CashflowSummaryData, error) {
	period := currentPeriodLabel()
	var out dashboarddomain.CashflowSummaryData
	out.Period = period
	q := r.db.WithContext(ctx).Table("cash_movements cm")
	q = applyCashflowBranchFilter(q, "cm", branchID)
	result := q.Select(`
			COALESCE(SUM(CASE WHEN cm.type = 'income' THEN cm.amount ELSE 0 END), 0) AS total_income,
			COALESCE(SUM(CASE WHEN cm.type = 'expense' THEN cm.amount ELSE 0 END), 0) AS total_expense,
			COALESCE(SUM(CASE WHEN cm.type = 'income' THEN cm.amount ELSE -cm.amount END), 0) AS balance
		`).
		Where("cm.org_id = ? AND cm.created_at >= ?", orgID, currentMonthStart()).
		Scan(&out)
	return out, result.Error
}

type quotesPipelineRow struct {
	Status string `gorm:"column:status"`
	Count  int64  `gorm:"column:count"`
}

func (r *Repository) LoadQuotesPipeline(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) (dashboarddomain.QuotesPipelineData, error) {
	var rows []quotesPipelineRow
	q := r.db.WithContext(ctx).Table("quotes q")
	q = applyEntityBranchFilter(q, "q", branchID)
	if err := q.Select("q.status AS status, COUNT(*) AS count").
		Where("q.org_id = ?", orgID).
		Group("q.status").
		Scan(&rows).Error; err != nil {
		return dashboarddomain.QuotesPipelineData{}, err
	}
	out := dashboarddomain.QuotesPipelineData{}
	for _, row := range rows {
		switch row.Status {
		case "draft":
			out.Draft = row.Count
		case "sent":
			out.Sent = row.Count
		case "accepted":
			out.Accepted = row.Count
		case "rejected":
			out.Rejected = row.Count
		}
	}
	out.PendingTotal = out.Draft + out.Sent
	return out, nil
}

func (r *Repository) LoadLowStock(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) (dashboarddomain.LowStockData, error) {
	out := dashboarddomain.LowStockData{}
	base := r.db.WithContext(ctx).Table("stock_levels sl")
	if normalized := normalizeBranchID(branchID); normalized != nil {
		base = base.Where("(sl.branch_id = ? OR sl.branch_id IS NULL)", *normalized)
	}
	subquery := base.Select(`
			sl.product_id,
			COALESCE(SUM(sl.quantity), 0) AS quantity,
			COALESCE(MAX(sl.min_quantity), 0) AS min_quantity
		`).
		Where("sl.org_id = ?", orgID).
		Group("sl.product_id")
	if err := r.db.WithContext(ctx).Table("(?) aggregated", subquery).
		Where("aggregated.min_quantity > 0 AND aggregated.quantity <= aggregated.min_quantity").
		Count(&out.Total).Error; err != nil {
		return dashboarddomain.LowStockData{}, err
	}
	if err := r.db.WithContext(ctx).Table("(?) aggregated", subquery).
		Select(`
			COALESCE(aggregated.product_id::text, '') AS product_id,
			COALESCE(p.name, '') AS product_name,
			COALESCE(p.sku, '') AS sku,
			aggregated.quantity,
			aggregated.min_quantity
		`).
		Joins("LEFT JOIN products p ON p.id = aggregated.product_id").
		Where("aggregated.min_quantity > 0 AND aggregated.quantity <= aggregated.min_quantity").
		Order("(aggregated.min_quantity - aggregated.quantity) DESC, p.name ASC").
		Limit(6).
		Scan(&out.Items).Error; err != nil {
		return dashboarddomain.LowStockData{}, err
	}
	return out, nil
}

func (r *Repository) LoadRecentSales(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) (dashboarddomain.RecentSalesData, error) {
	out := dashboarddomain.RecentSalesData{}
	q := r.db.WithContext(ctx).Table("sales s")
	q = applyEntityBranchFilter(q, "s", branchID)
	err := q.Select(`
			s.id::text AS id,
			s.number,
			COALESCE(s.party_name, '') AS customer_name,
			s.total,
			s.currency,
			to_char(s.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
		`).
		Where("s.org_id = ?", orgID).
		Order("s.created_at DESC").
		Limit(6).
		Scan(&out.Items).Error
	return out, err
}

func (r *Repository) LoadTopProducts(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) (dashboarddomain.TopProductsData, error) {
	out := dashboarddomain.TopProductsData{Period: currentPeriodLabel()}
	q := r.db.WithContext(ctx).Table("sale_items si").Joins("JOIN sales s ON s.id = si.sale_id")
	q = applyEntityBranchFilter(q, "s", branchID)
	err := q.Select(`
			COALESCE(si.product_id::text, '') AS product_id,
			si.description AS name,
			SUM(si.quantity) AS quantity,
			SUM(si.subtotal) AS total
		`).
		Where("s.org_id = ? AND s.created_at >= ? AND s.status = 'completed'", orgID, currentMonthStart()).
		Group("si.product_id, si.description").
		Order("SUM(si.subtotal) DESC").
		Limit(5).
		Scan(&out.Items).Error
	return out, err
}

func (r *Repository) LoadTopServices(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) (dashboarddomain.TopServicesData, error) {
	out := dashboarddomain.TopServicesData{Period: currentPeriodLabel()}
	q := r.db.WithContext(ctx).Table("sale_items si").
		Joins("JOIN sales s ON s.id = si.sale_id").
		Joins("LEFT JOIN services sv ON sv.id = si.service_id")
	q = applyEntityBranchFilter(q, "s", branchID)
	err := q.Select(`
			COALESCE(si.service_id::text, '') AS service_id,
			COALESCE(sv.name, si.description) AS name,
			SUM(si.quantity) AS quantity,
			SUM(si.subtotal) AS total
		`).
		Where("s.org_id = ? AND s.created_at >= ? AND s.status = 'completed' AND si.service_id IS NOT NULL", orgID, currentMonthStart()).
		Group("si.service_id, COALESCE(sv.name, si.description)").
		Order("SUM(si.subtotal) DESC").
		Limit(5).
		Scan(&out.Items).Error
	return out, err
}

type billingStatusRow struct {
	PlanCode   string    `gorm:"column:plan_code"`
	Status     string    `gorm:"column:billing_status"`
	HardLimits []byte    `gorm:"column:hard_limits"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (r *Repository) LoadBillingStatus(ctx context.Context, orgID uuid.UUID) (dashboarddomain.BillingStatusData, error) {
	var row billingStatusRow
	if err := r.db.WithContext(ctx).Table("tenant_settings").Where("org_id = ?", orgID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dashboarddomain.BillingStatusData{PlanCode: "starter", Status: "trialing", HardLimits: map[string]any{}}, nil
		}
		return dashboarddomain.BillingStatusData{}, err
	}
	updatedAt := row.UpdatedAt
	return dashboarddomain.BillingStatusData{
		PlanCode:   row.PlanCode,
		Status:     row.Status,
		HardLimits: decodeObject(row.HardLimits),
		UpdatedAt:  &updatedAt,
	}, nil
}

func (r *Repository) LoadAuditActivity(ctx context.Context, orgID uuid.UUID) (dashboarddomain.AuditActivityData, error) {
	out := dashboarddomain.AuditActivityData{}
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			id::text AS id,
			COALESCE(actor, '') AS actor,
			action,
			resource_type,
			COALESCE(resource_id, '') AS resource_id,
			to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
		FROM audit_log
		WHERE org_id = ?
		ORDER BY created_at DESC
		LIMIT 6
	`, orgID).Scan(&out.Items).Error
	return out, err
}

func decodeObject(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func currentMonthStart() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func currentPeriodLabel() string {
	return currentMonthStart().Format("2006-01")
}
