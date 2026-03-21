package currency

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/devpablocristo/core/backend/go/pagination"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/currency/repository/models"
	currencydomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/currency/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ListLatest(ctx context.Context, orgID uuid.UUID, fromCurrency, toCurrency string, limit int) ([]currencydomain.ExchangeRate, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&models.ExchangeRateModel{}).Where("org_id = ?", orgID)
	if fromCurrency != "" {
		q = q.Where("from_currency = ?", fromCurrency)
	}
	if toCurrency != "" {
		q = q.Where("to_currency = ?", toCurrency)
	}
	var rows []models.ExchangeRateModel
	if err := q.Order("rate_date DESC").Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]currencydomain.ExchangeRate, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) Upsert(ctx context.Context, in currencydomain.ExchangeRate) (currencydomain.ExchangeRate, error) {
	row := models.ExchangeRateModel{
		ID:           in.ID,
		OrgID:        in.OrgID,
		FromCurrency: in.FromCurrency,
		ToCurrency:   in.ToCurrency,
		RateType:     in.RateType,
		BuyRate:      in.BuyRate,
		SellRate:     in.SellRate,
		Source:       in.Source,
		RateDate:     time.Date(in.RateDate.Year(), in.RateDate.Month(), in.RateDate.Day(), 0, 0, 0, 0, time.UTC),
		CreatedAt:    time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "org_id"}, {Name: "from_currency"}, {Name: "to_currency"}, {Name: "rate_type"}, {Name: "rate_date"}},
		DoUpdates: clause.Assignments(map[string]any{
			"buy_rate":   row.BuyRate,
			"sell_rate":  row.SellRate,
			"source":     row.Source,
			"created_at": row.CreatedAt,
		}),
	}).Create(&row).Error; err != nil {
		return currencydomain.ExchangeRate{}, err
	}
	return toDomain(row), nil
}

func toDomain(row models.ExchangeRateModel) currencydomain.ExchangeRate {
	return currencydomain.ExchangeRate{
		ID:           row.ID,
		OrgID:        row.OrgID,
		FromCurrency: row.FromCurrency,
		ToCurrency:   row.ToCurrency,
		RateType:     row.RateType,
		BuyRate:      row.BuyRate,
		SellRate:     row.SellRate,
		Source:       row.Source,
		RateDate:     row.RateDate,
		CreatedAt:    row.CreatedAt,
	}
}
