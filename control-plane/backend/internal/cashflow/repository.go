package cashflow

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/cashflow/repository/models"
	cashdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/cashflow/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type ListParams struct {
	OrgID    uuid.UUID
	Limit    int
	After    *uuid.UUID
	Type     string
	Category string
	From     *time.Time
	To       *time.Time
}

func (r *Repository) List(ctx context.Context, p ListParams) ([]cashdomain.CashMovement, int64, bool, *uuid.UUID, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	q := r.db.WithContext(ctx).Model(&models.CashMovementModel{}).Where("org_id = ?", p.OrgID)
	if t := strings.TrimSpace(p.Type); t != "" {
		q = q.Where("type = ?", t)
	}
	if c := strings.TrimSpace(p.Category); c != "" {
		q = q.Where("category = ?", c)
	}
	if p.From != nil {
		q = q.Where("created_at >= ?", *p.From)
	}
	if p.To != nil {
		q = q.Where("created_at <= ?", *p.To)
	}
	if p.After != nil && *p.After != uuid.Nil {
		q = q.Where("id < ?", *p.After)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}
	var rows []models.CashMovementModel
	if err := q.Order("created_at DESC").Order("id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]cashdomain.CashMovement, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	var next *uuid.UUID
	if hasMore && len(rows) > 0 {
		v := rows[len(rows)-1].ID
		next = &v
	}
	return out, total, hasMore, next, nil
}

func (r *Repository) Create(ctx context.Context, in cashdomain.CashMovement) (cashdomain.CashMovement, error) {
	row := models.CashMovementModel{
		ID:            uuid.New(),
		OrgID:         in.OrgID,
		Type:          in.Type,
		Amount:        in.Amount,
		Currency:      in.Currency,
		Category:      in.Category,
		Description:   in.Description,
		PaymentMethod: in.PaymentMethod,
		ReferenceType: in.ReferenceType,
		ReferenceID:   in.ReferenceID,
		CreatedBy:     in.CreatedBy,
		CreatedAt:     time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return cashdomain.CashMovement{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetCurrency(ctx context.Context, orgID uuid.UUID) string {
	var v string
	if err := r.db.WithContext(ctx).Table("tenant_settings").Select("currency").Where("org_id = ?", orgID).Take(&v).Error; err != nil || strings.TrimSpace(v) == "" {
		return "ARS"
	}
	return strings.TrimSpace(v)
}

func (r *Repository) Summary(ctx context.Context, orgID uuid.UUID, from, to time.Time) (cashdomain.CashSummary, error) {
	type row struct {
		Income  float64 `gorm:"column:income"`
		Expense float64 `gorm:"column:expense"`
	}
	var agg row
	if err := r.db.WithContext(ctx).Table("cash_movements").
		Select("COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END),0) as income, COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END),0) as expense").
		Where("org_id = ? AND created_at >= ? AND created_at <= ?", orgID, from, to).
		Take(&agg).Error; err != nil {
		return cashdomain.CashSummary{}, err
	}
	cur := r.GetCurrency(ctx, orgID)
	return cashdomain.CashSummary{OrgID: orgID, PeriodStart: from, PeriodEnd: to, TotalIncome: agg.Income, TotalExpense: agg.Expense, Balance: agg.Income - agg.Expense, Currency: cur}, nil
}

func (r *Repository) DailySummary(ctx context.Context, orgID uuid.UUID, days int) ([]cashdomain.CashSummary, error) {
	if days <= 0 {
		days = 30
	}
	start := time.Now().UTC().Truncate(24 * time.Hour).AddDate(0, 0, -(days - 1))
	cur := r.GetCurrency(ctx, orgID)

	type dailyRow struct {
		Day     time.Time `gorm:"column:day"`
		Income  float64   `gorm:"column:income"`
		Expense float64   `gorm:"column:expense"`
	}
	var rows []dailyRow
	if err := r.db.WithContext(ctx).
		Table("cash_movements").
		Select("date_trunc('day', created_at) as day, COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END),0) as income, COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END),0) as expense").
		Where("org_id = ? AND created_at >= ?", orgID, start).
		Group("day").
		Order("day ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	byDay := make(map[string]dailyRow, len(rows))
	for _, row := range rows {
		byDay[row.Day.Format("2006-01-02")] = row
	}

	result := make([]cashdomain.CashSummary, 0, days)
	for i := 0; i < days; i++ {
		day := start.AddDate(0, 0, i)
		key := day.Format("2006-01-02")
		dr := byDay[key]
		result = append(result, cashdomain.CashSummary{
			OrgID:        orgID,
			PeriodStart:  day,
			PeriodEnd:    day.Add(24*time.Hour - time.Nanosecond),
			TotalIncome:  dr.Income,
			TotalExpense: dr.Expense,
			Balance:      dr.Income - dr.Expense,
			Currency:     cur,
		})
	}
	return result, nil
}

func toDomain(row models.CashMovementModel) cashdomain.CashMovement {
	return cashdomain.CashMovement{
		ID:            row.ID,
		OrgID:         row.OrgID,
		Type:          row.Type,
		Amount:        row.Amount,
		Currency:      row.Currency,
		Category:      row.Category,
		Description:   row.Description,
		PaymentMethod: row.PaymentMethod,
		ReferenceType: row.ReferenceType,
		ReferenceID:   row.ReferenceID,
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt,
	}
}
