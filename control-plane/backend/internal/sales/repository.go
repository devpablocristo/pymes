package sales

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/sales/repository/models"
	saledomain "github.com/devpablocristo/pymes/control-plane/backend/internal/sales/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type tenantBusinessSettings struct {
	Currency       string  `gorm:"column:currency"`
	TaxRate        float64 `gorm:"column:tax_rate"`
	SalePrefix     string  `gorm:"column:sale_prefix"`
	NextSaleNumber int     `gorm:"column:next_sale_number"`
}

type ProductSnapshot struct {
	ID         uuid.UUID
	Name       string
	Price      float64
	CostPrice  float64
	TaxRate    *float64
	TrackStock bool
}

func (r *Repository) GetProductSnapshot(ctx context.Context, orgID, productID uuid.UUID) (ProductSnapshot, error) {
	var row ProductSnapshot
	err := r.db.WithContext(ctx).
		Table("products").
		Select("id, name, price, cost_price, tax_rate, track_stock").
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, productID).
		Take(&row).Error
	if err != nil {
		return ProductSnapshot{}, err
	}
	return row, nil
}

func (r *Repository) GetTenantSettings(ctx context.Context, orgID uuid.UUID) (currency string, taxRate float64, salePrefix string, err error) {
	var row tenantBusinessSettings
	err = r.db.WithContext(ctx).
		Table("tenant_settings").
		Select("currency, tax_rate, sale_prefix, next_sale_number").
		Where("org_id = ?", orgID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "ARS", 21.0, "VTA", nil
		}
		return "", 0, "", err
	}
	return normalizeSettings(row).Currency, normalizeSettings(row).TaxRate, normalizeSettings(row).SalePrefix, nil
}

type CreateItemInput struct {
	ProductID   *uuid.UUID
	Description string
	Quantity    float64
	UnitPrice   float64
	CostPrice   float64
	TaxRate     float64
	Subtotal    float64
	SortOrder   int
}

type CreateInput struct {
	OrgID         uuid.UUID
	CustomerID    *uuid.UUID
	CustomerName  string
	QuoteID       *uuid.UUID
	PaymentMethod string
	Subtotal      float64
	TaxTotal      float64
	Total         float64
	Currency      string
	Notes         string
	CreatedBy     string
	Items         []CreateItemInput
}

func (r *Repository) Create(ctx context.Context, in CreateInput) (saledomain.Sale, error) {
	var out saledomain.Sale
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tenant, err := r.getOrCreateTenantSettingsForUpdate(ctx, tx, in.OrgID)
		if err != nil {
			return err
		}

		number := fmt.Sprintf("%s-%05d", tenant.SalePrefix, tenant.NextSaleNumber)

		saleRow := models.SaleModel{
			ID:            uuid.New(),
			OrgID:         in.OrgID,
			Number:        number,
			CustomerID:    in.CustomerID,
			CustomerName:  strings.TrimSpace(in.CustomerName),
			QuoteID:       in.QuoteID,
			Status:        "completed",
			PaymentMethod: coalesce(in.PaymentMethod, "cash"),
			Subtotal:      in.Subtotal,
			TaxTotal:      in.TaxTotal,
			Total:         in.Total,
			Currency:      coalesce(in.Currency, tenant.Currency),
			Notes:         strings.TrimSpace(in.Notes),
			CreatedBy:     strings.TrimSpace(in.CreatedBy),
			CreatedAt:     time.Now().UTC(),
		}
		if err := tx.Create(&saleRow).Error; err != nil {
			return err
		}

		itemRows := make([]models.SaleItemModel, 0, len(in.Items))
		for _, item := range in.Items {
			itemRows = append(itemRows, models.SaleItemModel{
				ID:          uuid.New(),
				SaleID:      saleRow.ID,
				ProductID:   item.ProductID,
				Description: strings.TrimSpace(item.Description),
				Quantity:    item.Quantity,
				UnitPrice:   item.UnitPrice,
				CostPrice:   item.CostPrice,
				TaxRate:     item.TaxRate,
				Subtotal:    item.Subtotal,
				SortOrder:   item.SortOrder,
			})
		}
		if len(itemRows) > 0 {
			if err := tx.Create(&itemRows).Error; err != nil {
				return err
			}
		}

		if err := tx.Table("tenant_settings").
			Where("org_id = ?", in.OrgID).
			Updates(map[string]any{
				"next_sale_number": tenant.NextSaleNumber + 1,
				"updated_at":       gorm.Expr("now()"),
			}).Error; err != nil {
			return err
		}

		out = saleToDomain(saleRow, itemRows)
		return nil
	})
	if err != nil {
		return saledomain.Sale{}, err
	}
	return out, nil
}

type ListParams struct {
	OrgID         uuid.UUID
	Limit         int
	After         *uuid.UUID
	CustomerID    *uuid.UUID
	PaymentMethod string
	From          *time.Time
	To            *time.Time
}

func (r *Repository) List(ctx context.Context, p ListParams) ([]saledomain.Sale, int64, bool, *uuid.UUID, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	q := r.db.WithContext(ctx).Model(&models.SaleModel{}).Where("org_id = ?", p.OrgID)
	if p.CustomerID != nil && *p.CustomerID != uuid.Nil {
		q = q.Where("party_id = ?", *p.CustomerID)
	}
	if pm := strings.TrimSpace(p.PaymentMethod); pm != "" {
		q = q.Where("payment_method = ?", pm)
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

	var rows []models.SaleModel
	if err := q.Order("id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	out := make([]saledomain.Sale, 0, len(rows))
	for _, row := range rows {
		out = append(out, saleToDomain(row, nil))
	}

	var next *uuid.UUID
	if hasMore && len(rows) > 0 {
		v := rows[len(rows)-1].ID
		next = &v
	}
	return out, total, hasMore, next, nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, saleID uuid.UUID) (saledomain.Sale, error) {
	var saleRow models.SaleModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, saleID).Take(&saleRow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return saledomain.Sale{}, gorm.ErrRecordNotFound
		}
		return saledomain.Sale{}, err
	}
	var itemRows []models.SaleItemModel
	if err := r.db.WithContext(ctx).Where("sale_id = ?", saleID).Order("sort_order ASC").Find(&itemRows).Error; err != nil {
		return saledomain.Sale{}, err
	}
	return saleToDomain(saleRow, itemRows), nil
}

func (r *Repository) Void(ctx context.Context, orgID, saleID uuid.UUID) (saledomain.Sale, error) {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&models.SaleModel{}).
		Where("org_id = ? AND id = ? AND status <> 'voided'", orgID, saleID).
		Updates(map[string]any{"status": "voided", "voided_at": now})
	if res.Error != nil {
		return saledomain.Sale{}, res.Error
	}
	if res.RowsAffected == 0 {
		// Either already voided or does not exist. Fetch to disambiguate.
		out, err := r.GetByID(ctx, orgID, saleID)
		if err != nil {
			return saledomain.Sale{}, err
		}
		return out, nil
	}
	return r.GetByID(ctx, orgID, saleID)
}

func (r *Repository) getOrCreateTenantSettingsForUpdate(ctx context.Context, tx *gorm.DB, orgID uuid.UUID) (tenantBusinessSettings, error) {
	var tenant tenantBusinessSettings
	err := tx.WithContext(ctx).
		Table("tenant_settings").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("currency, tax_rate, sale_prefix, next_sale_number").
		Where("org_id = ?", orgID).
		Take(&tenant).Error
	if err == nil {
		tenant = normalizeSettings(tenant)
		return r.syncNextSaleNumber(ctx, tx, orgID, tenant)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return tenantBusinessSettings{}, err
	}

	// Bootstrap tenant settings if missing for legacy/seed orgs.
	if err := tx.WithContext(ctx).Exec(`
		INSERT INTO tenant_settings (
			org_id, plan_code, hard_limits, currency, tax_rate, quote_prefix, sale_prefix,
			next_quote_number, next_sale_number, allow_negative_stock, created_at, updated_at
		)
		VALUES (?, 'starter', '{}'::jsonb, 'ARS', 21.0, 'PRE', 'VTA', 1, 1, true, now(), now())
		ON CONFLICT (org_id) DO NOTHING
	`, orgID).Error; err != nil {
		return tenantBusinessSettings{}, err
	}

	if err := tx.WithContext(ctx).
		Table("tenant_settings").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("currency, tax_rate, sale_prefix, next_sale_number").
		Where("org_id = ?", orgID).
		Take(&tenant).Error; err != nil {
		return tenantBusinessSettings{}, err
	}
	tenant = normalizeSettings(tenant)
	return r.syncNextSaleNumber(ctx, tx, orgID, tenant)
}

func (r *Repository) syncNextSaleNumber(ctx context.Context, tx *gorm.DB, orgID uuid.UUID, tenant tenantBusinessSettings) (tenantBusinessSettings, error) {
	pattern := fmt.Sprintf("%s-%%", tenant.SalePrefix)
	var maxExisting int
	if err := tx.WithContext(ctx).
		Table("sales").
		Select("COALESCE(MAX(CAST(right(number, 5) AS INTEGER)), 0)").
		Where("org_id = ? AND number LIKE ?", orgID, pattern).
		Scan(&maxExisting).Error; err != nil {
		return tenantBusinessSettings{}, err
	}
	if tenant.NextSaleNumber <= maxExisting {
		tenant.NextSaleNumber = maxExisting + 1
		if err := tx.WithContext(ctx).Table("tenant_settings").
			Where("org_id = ?", orgID).
			Updates(map[string]any{
				"next_sale_number": tenant.NextSaleNumber,
				"updated_at":       gorm.Expr("now()"),
			}).Error; err != nil {
			return tenantBusinessSettings{}, err
		}
	}
	return tenant, nil
}

func normalizeSettings(in tenantBusinessSettings) tenantBusinessSettings {
	out := in
	if strings.TrimSpace(out.Currency) == "" {
		out.Currency = "ARS"
	}
	if out.TaxRate <= 0 {
		out.TaxRate = 21.0
	}
	if strings.TrimSpace(out.SalePrefix) == "" {
		out.SalePrefix = "VTA"
	}
	if out.NextSaleNumber <= 0 {
		out.NextSaleNumber = 1
	}
	return out
}

func saleToDomain(saleRow models.SaleModel, itemRows []models.SaleItemModel) saledomain.Sale {
	items := make([]saledomain.SaleItem, 0, len(itemRows))
	for _, row := range itemRows {
		items = append(items, saledomain.SaleItem{
			ID:          row.ID,
			SaleID:      row.SaleID,
			ProductID:   row.ProductID,
			Description: row.Description,
			Quantity:    row.Quantity,
			UnitPrice:   row.UnitPrice,
			CostPrice:   row.CostPrice,
			TaxRate:     row.TaxRate,
			Subtotal:    row.Subtotal,
			SortOrder:   row.SortOrder,
		})
	}
	return saledomain.Sale{
		ID:            saleRow.ID,
		OrgID:         saleRow.OrgID,
		Number:        saleRow.Number,
		CustomerID:    saleRow.CustomerID,
		CustomerName:  saleRow.CustomerName,
		QuoteID:       saleRow.QuoteID,
		Status:        saleRow.Status,
		PaymentMethod: saleRow.PaymentMethod,
		Items:         items,
		Subtotal:      saleRow.Subtotal,
		TaxTotal:      saleRow.TaxTotal,
		Total:         saleRow.Total,
		Currency:      saleRow.Currency,
		Notes:         saleRow.Notes,
		CreatedBy:     saleRow.CreatedBy,
		CreatedAt:     saleRow.CreatedAt,
		VoidedAt:      saleRow.VoidedAt,
	}
}

func coalesce(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}
