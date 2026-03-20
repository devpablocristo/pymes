// Package dashboard provides persistence for dashboard layouts and widget data.
package dashboard

import (
	"errors"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	dashboarddomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/dashboard/usecases/domain"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type widgetRow struct {
	WidgetKey             string `gorm:"column:widget_key"`
	Title                 string `gorm:"column:title"`
	Description           string `gorm:"column:description"`
	Domain                string `gorm:"column:domain"`
	Kind                  string `gorm:"column:kind"`
	DefaultWidth          int    `gorm:"column:default_width"`
	DefaultHeight         int    `gorm:"column:default_height"`
	MinWidth              int    `gorm:"column:min_width"`
	MinHeight             int    `gorm:"column:min_height"`
	MaxWidth              int    `gorm:"column:max_width"`
	MaxHeight             int    `gorm:"column:max_height"`
	AllowedRolesJSON      []byte `gorm:"column:allowed_roles_json"`
	RequiredScopesJSON    []byte `gorm:"column:required_scopes_json"`
	SupportedContextsJSON []byte `gorm:"column:supported_contexts_json"`
	SettingsSchemaJSON    []byte `gorm:"column:settings_schema_json"`
	DataEndpoint          string `gorm:"column:data_endpoint"`
	IsActive              bool   `gorm:"column:is_active"`
}

type defaultLayoutRow struct {
	LayoutKey string `gorm:"column:layout_key"`
	Context   string `gorm:"column:context"`
	Name      string `gorm:"column:name"`
	ItemsJSON []byte `gorm:"column:items_json"`
}

type userLayoutRow struct {
	UserID                      *uuid.UUID `gorm:"column:user_id"`
	UserActor                   string     `gorm:"column:user_actor"`
	Context                     string     `gorm:"column:context"`
	LayoutVersion               int        `gorm:"column:layout_version"`
	ItemsJSON                   []byte     `gorm:"column:items_json"`
	LastAppliedDefaultLayoutKey string     `gorm:"column:last_applied_default_layout_key"`
	CreatedAt                   time.Time  `gorm:"column:created_at"`
	UpdatedAt                   time.Time  `gorm:"column:updated_at"`
}

func (r *Repository) ListWidgets(ctx context.Context) ([]dashboarddomain.WidgetDefinition, error) {
	var rows []widgetRow
	if err := r.db.WithContext(ctx).
		Table("dashboard_widgets_catalog").
		Where("is_active = ?", true).
		Order("title ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]dashboarddomain.WidgetDefinition, 0, len(rows))
	for _, row := range rows {
		widget := dashboarddomain.WidgetDefinition{
			WidgetKey:    row.WidgetKey,
			Title:        row.Title,
			Description:  row.Description,
			Domain:       row.Domain,
			Kind:         row.Kind,
			DefaultSize:  dashboarddomain.WidgetSize{W: row.DefaultWidth, H: row.DefaultHeight},
			MinW:         row.MinWidth,
			MinH:         row.MinHeight,
			MaxW:         row.MaxWidth,
			MaxH:         row.MaxHeight,
			DataEndpoint: row.DataEndpoint,
			Status:       boolStatus(row.IsActive),
		}
		widget.AllowedRoles = decodeStringSlice(row.AllowedRolesJSON)
		widget.RequiredScopes = decodeStringSlice(row.RequiredScopesJSON)
		widget.SupportedContexts = decodeStringSlice(row.SupportedContextsJSON)
		widget.SettingsSchema = decodeObject(row.SettingsSchemaJSON)
		out = append(out, widget)
	}
	return out, nil
}

func (r *Repository) GetDefaultLayout(ctx context.Context, contextKey string) (dashboarddomain.DefaultLayout, error) {
	var row defaultLayoutRow
	if err := r.db.WithContext(ctx).
		Table("dashboard_default_layouts").
		Where("context = ? AND is_active = ?", contextKey, true).
		Take(&row).Error; err != nil {
		return dashboarddomain.DefaultLayout{}, err
	}
	return dashboarddomain.DefaultLayout{
		LayoutKey: row.LayoutKey,
		Context:   row.Context,
		Name:      row.Name,
		Items:     decodeLayoutItems(row.ItemsJSON),
	}, nil
}

func (r *Repository) GetUserLayout(ctx context.Context, actor, contextKey string) (dashboarddomain.UserLayout, bool, error) {
	var row userLayoutRow
	err := r.db.WithContext(ctx).
		Table("user_dashboard_layouts").
		Where("user_actor = ? AND context = ?", actor, contextKey).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dashboarddomain.UserLayout{}, false, nil
		}
		return dashboarddomain.UserLayout{}, false, err
	}
	return dashboarddomain.UserLayout{
		UserID:                      row.UserID,
		UserActor:                   row.UserActor,
		Context:                     row.Context,
		LayoutVersion:               row.LayoutVersion,
		Items:                       decodeLayoutItems(row.ItemsJSON),
		LastAppliedDefaultLayoutKey: row.LastAppliedDefaultLayoutKey,
		CreatedAt:                   row.CreatedAt,
		UpdatedAt:                   row.UpdatedAt,
	}, true, nil
}

func (r *Repository) SaveUserLayout(ctx context.Context, in dashboarddomain.UserLayout) error {
	itemsJSON, err := json.Marshal(in.Items)
	if err != nil {
		return fmt.Errorf("marshal layout items: %w", err)
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"user_id":                         in.UserID,
		"items_json":                      itemsJSON,
		"layout_version":                  in.LayoutVersion,
		"last_applied_default_layout_key": in.LastAppliedDefaultLayoutKey,
		"updated_at":                      now,
	}
	result := r.db.WithContext(ctx).
		Table("user_dashboard_layouts").
		Where("user_actor = ? AND context = ?", in.UserActor, in.Context).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	return r.db.WithContext(ctx).Table("user_dashboard_layouts").Create(map[string]any{
		"id":                              uuid.New(),
		"user_id":                         in.UserID,
		"user_actor":                      in.UserActor,
		"context":                         in.Context,
		"layout_version":                  in.LayoutVersion,
		"items_json":                      itemsJSON,
		"last_applied_default_layout_key": in.LastAppliedDefaultLayoutKey,
		"created_at":                      now,
		"updated_at":                      now,
	}).Error
}

func (r *Repository) ResetUserLayout(ctx context.Context, actor, contextKey string) error {
	return r.db.WithContext(ctx).Table("user_dashboard_layouts").Where("user_actor = ? AND context = ?", actor, contextKey).Delete(nil).Error
}

func (r *Repository) ResolveUserID(ctx context.Context, actor string) (*uuid.UUID, error) {
	if strings.TrimSpace(actor) == "" {
		return nil, nil
	}
	var row struct {
		ID uuid.UUID `gorm:"column:id"`
	}
	err := r.db.WithContext(ctx).Table("users").Select("id").Where("external_id = ? AND deleted_at IS NULL", actor).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row.ID, nil
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

func (r *Repository) LoadQuotesPipeline(ctx context.Context, orgID uuid.UUID) (dashboarddomain.QuotesPipelineData, error) {
	var rows []struct {
		Status string `gorm:"column:status"`
		Count  int64  `gorm:"column:count"`
	}
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
		switch strings.ToLower(strings.TrimSpace(row.Status)) {
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
	customerNameExpr, err := r.salesPartyNameExpr(ctx)
	if err != nil {
		return dashboarddomain.RecentSalesData{}, err
	}
	err = r.db.WithContext(ctx).Raw(fmt.Sprintf(`
		SELECT
			id::text AS id,
			number,
			%s AS customer_name,
			total,
			currency,
			to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
		FROM sales
		WHERE org_id = ?
		ORDER BY created_at DESC
		LIMIT 6
	`, customerNameExpr), orgID).Scan(&out.Items).Error
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

func (r *Repository) LoadBillingStatus(ctx context.Context, orgID uuid.UUID) (dashboarddomain.BillingStatusData, error) {
	var row struct {
		PlanCode   string    `gorm:"column:plan_code"`
		Status     string    `gorm:"column:billing_status"`
		HardLimits []byte    `gorm:"column:hard_limits"`
		UpdatedAt  time.Time `gorm:"column:updated_at"`
	}
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

func decodeLayoutItems(raw []byte) []dashboarddomain.LayoutItem {
	if len(raw) == 0 {
		return []dashboarddomain.LayoutItem{}
	}
	var items []dashboarddomain.LayoutItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return []dashboarddomain.LayoutItem{}
	}
	for idx := range items {
		if items[idx].Settings == nil {
			items[idx].Settings = map[string]any{}
		}
	}
	return items
}

func decodeStringSlice(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
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

func (r *Repository) salesPartyNameExpr(ctx context.Context) (string, error) {
	hasPartyName, err := r.tableHasColumn(ctx, "sales", "party_name")
	if err != nil {
		return "", err
	}
	if hasPartyName {
		return "COALESCE(party_name, '')", nil
	}
	hasCustomerName, err := r.tableHasColumn(ctx, "sales", "customer_name")
	if err != nil {
		return "", err
	}
	if hasCustomerName {
		return "COALESCE(customer_name, '')", nil
	}
	return "''", nil
}

func (r *Repository) tableHasColumn(ctx context.Context, tableName, columnName string) (bool, error) {
	var exists bool
	err := r.db.WithContext(ctx).Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = ?
			  AND column_name = ?
		)
	`, tableName, columnName).Scan(&exists).Error
	return exists, err
}

func boolStatus(active bool) string {
	if active {
		return "active"
	}
	return "inactive"
}

func currentMonthStart() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func currentPeriodLabel() string {
	return currentMonthStart().Format("2006-01")
}
