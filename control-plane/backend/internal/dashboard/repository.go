package dashboard

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	dashboarddomain "github.com/devpablocristo/pymes/control-plane/backend/internal/dashboard/usecases/domain"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Get(ctx context.Context, orgID uuid.UUID) (dashboarddomain.Dashboard, error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	out := dashboarddomain.Dashboard{}
	r.db.WithContext(ctx).Table("sales").Select("COALESCE(SUM(total),0)").Where("org_id = ? AND created_at >= ? AND status = 'completed'", orgID, today).Scan(&out.SalesToday)
	r.db.WithContext(ctx).Table("sales").Select("COALESCE(SUM(total),0)").Where("org_id = ? AND created_at >= ? AND status = 'completed'", orgID, monthStart).Scan(&out.SalesMonth)
	r.db.WithContext(ctx).Raw(`SELECT COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE -amount END), 0) FROM cash_movements WHERE org_id = ?`, orgID).Scan(&out.CashflowBalance)
	r.db.WithContext(ctx).Table("quotes").Where("org_id = ? AND status IN ('draft','sent')", orgID).Count(&out.PendingQuotes)
	r.db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM stock_levels WHERE org_id = ? AND min_quantity > 0 AND quantity <= min_quantity`, orgID).Scan(&out.LowStockProducts)
	_ = r.db.WithContext(ctx).Raw(`
		SELECT COALESCE(si.product_id::text, '' ) AS product_id, si.description AS name, SUM(si.quantity) AS quantity, SUM(si.subtotal) AS total
		FROM sale_items si
		JOIN sales s ON s.id = si.sale_id
		WHERE s.org_id = ? AND s.created_at >= ? AND s.status = 'completed'
		GROUP BY si.product_id, si.description
		ORDER BY SUM(si.subtotal) DESC
		LIMIT 5
	`, orgID, monthStart).Scan(&out.TopProducts).Error
	_ = r.db.WithContext(ctx).Raw(`
		SELECT id::text as id, number, COALESCE(party_name, '') AS customer_name, total, currency, to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at
		FROM sales
		WHERE org_id = ?
		ORDER BY created_at DESC
		LIMIT 5
	`, orgID).Scan(&out.RecentSales).Error
	return out, nil
}
