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

func (r *Repository) ListWidgets(ctx context.Context) ([]dashboarddomain.WidgetDefinition, error) {
	_ = ctx
	catalog := fixedDashboardWidgets()
	out := make([]dashboarddomain.WidgetDefinition, 0, len(catalog))
	for _, widget := range catalog {
		out = append(out, widget)
	}
	return out, nil
}

func (r *Repository) LoadSalesSummary(ctx context.Context, orgID uuid.UUID) (dashboarddomain.SalesSummaryData, error) {
	period := currentPeriodLabel()
	var out dashboarddomain.SalesSummaryData
	out.Period = period
	result := r.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(SUM(total), 0) AS total_sales,
			COUNT(*) AS count_sales,
			COALESCE(AVG(total), 0) AS average_ticket
		FROM sales
		WHERE org_id = ? AND status = 'completed' AND created_at >= ?
	`, orgID, currentMonthStart()).Scan(&out)
	return out, result.Error
}

func (r *Repository) LoadCashflowSummary(ctx context.Context, orgID uuid.UUID) (dashboarddomain.CashflowSummaryData, error) {
	period := currentPeriodLabel()
	var out dashboarddomain.CashflowSummaryData
	out.Period = period
	result := r.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END), 0) AS total_income,
			COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0) AS total_expense,
			COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE -amount END), 0) AS balance
		FROM cash_movements
		WHERE org_id = ? AND created_at >= ?
	`, orgID, currentMonthStart()).Scan(&out)
	return out, result.Error
}

type quotesPipelineRow struct {
	Status string `gorm:"column:status"`
	Count  int64  `gorm:"column:count"`
}

func (r *Repository) LoadQuotesPipeline(ctx context.Context, orgID uuid.UUID) (dashboarddomain.QuotesPipelineData, error) {
	var rows []quotesPipelineRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT status, COUNT(*) AS count
		FROM quotes
		WHERE org_id = ?
		GROUP BY status
	`, orgID).Scan(&rows).Error; err != nil {
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

func (r *Repository) LoadLowStock(ctx context.Context, orgID uuid.UUID) (dashboarddomain.LowStockData, error) {
	out := dashboarddomain.LowStockData{}
	if err := r.db.WithContext(ctx).Raw(`
		SELECT COUNT(*)
		FROM stock_levels
		WHERE org_id = ? AND min_quantity > 0 AND quantity <= min_quantity
	`, orgID).Scan(&out.Total).Error; err != nil {
		return dashboarddomain.LowStockData{}, err
	}
	if err := r.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(sl.product_id::text, '') AS product_id,
			COALESCE(p.name, '') AS product_name,
			COALESCE(p.sku, '') AS sku,
			sl.quantity,
			sl.min_quantity
		FROM stock_levels sl
		LEFT JOIN products p ON p.id = sl.product_id
		WHERE sl.org_id = ? AND sl.min_quantity > 0 AND sl.quantity <= sl.min_quantity
		ORDER BY (sl.min_quantity - sl.quantity) DESC, p.name ASC
		LIMIT 6
	`, orgID).Scan(&out.Items).Error; err != nil {
		return dashboarddomain.LowStockData{}, err
	}
	return out, nil
}

func (r *Repository) LoadRecentSales(ctx context.Context, orgID uuid.UUID) (dashboarddomain.RecentSalesData, error) {
	out := dashboarddomain.RecentSalesData{}
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			id::text AS id,
			number,
			COALESCE(party_name, '') AS customer_name,
			total,
			currency,
			to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
		FROM sales
		WHERE org_id = ?
		ORDER BY created_at DESC
		LIMIT 6
	`, orgID).Scan(&out.Items).Error
	return out, err
}

func (r *Repository) LoadTopProducts(ctx context.Context, orgID uuid.UUID) (dashboarddomain.TopProductsData, error) {
	out := dashboarddomain.TopProductsData{Period: currentPeriodLabel()}
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(si.product_id::text, '') AS product_id,
			si.description AS name,
			SUM(si.quantity) AS quantity,
			SUM(si.subtotal) AS total
		FROM sale_items si
		JOIN sales s ON s.id = si.sale_id
		WHERE s.org_id = ? AND s.created_at >= ? AND s.status = 'completed'
		GROUP BY si.product_id, si.description
		ORDER BY SUM(si.subtotal) DESC
		LIMIT 5
	`, orgID, currentMonthStart()).Scan(&out.Items).Error
	return out, err
}

func (r *Repository) LoadTopServices(ctx context.Context, orgID uuid.UUID) (dashboarddomain.TopServicesData, error) {
	out := dashboarddomain.TopServicesData{Period: currentPeriodLabel()}
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(si.service_id::text, '') AS service_id,
			COALESCE(sv.name, si.description) AS name,
			SUM(si.quantity) AS quantity,
			SUM(si.subtotal) AS total
		FROM sale_items si
		JOIN sales s ON s.id = si.sale_id
		LEFT JOIN services sv ON sv.id = si.service_id
		WHERE s.org_id = ? AND s.created_at >= ? AND s.status = 'completed' AND si.service_id IS NOT NULL
		GROUP BY si.service_id, COALESCE(sv.name, si.description)
		ORDER BY SUM(si.subtotal) DESC
		LIMIT 5
	`, orgID, currentMonthStart()).Scan(&out.Items).Error
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
