// Package quotes implements quote persistence and domain adapters.
package quotes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/devpablocristo/core/http/go/pagination"
	utils "github.com/devpablocristo/core/validate/go/stringutil"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/quotes/repository/models"
	quotedomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/quotes/usecases/domain"
)

var ErrQuoteNotDraft = domainerr.Conflict("quote is not in draft status")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type tenantBusinessSettings struct {
	Currency        string  `gorm:"column:currency"`
	TaxRate         float64 `gorm:"column:tax_rate"`
	QuotePrefix     string  `gorm:"column:quote_prefix"`
	NextQuoteNumber int     `gorm:"column:next_quote_number"`
}

type ProductSnapshot struct {
	ID      uuid.UUID
	Name    string
	Price   float64
	TaxRate *float64
}

type ServiceSnapshot struct {
	ID        uuid.UUID
	Name      string
	Price     float64
	CostPrice float64
	TaxRate   *float64
}

func (r *Repository) GetProductSnapshot(ctx context.Context, orgID, productID uuid.UUID) (ProductSnapshot, error) {
	var row ProductSnapshot
	err := r.db.WithContext(ctx).
		Table("products").
		Select("id, name, price, tax_rate").
		Where("org_id = ? AND id = ? AND deleted_at IS NULL AND is_active = true", orgID, productID).
		Take(&row).Error
	if err != nil {
		return ProductSnapshot{}, err
	}
	return row, nil
}

func (r *Repository) GetServiceSnapshot(ctx context.Context, orgID, serviceID uuid.UUID) (ServiceSnapshot, error) {
	var row ServiceSnapshot
	err := r.db.WithContext(ctx).
		Table("services").
		Select("id, name, sale_price as price, cost_price, tax_rate").
		Where("org_id = ? AND id = ? AND deleted_at IS NULL AND is_active = true", orgID, serviceID).
		Take(&row).Error
	if err != nil {
		return ServiceSnapshot{}, err
	}
	return row, nil
}

func (r *Repository) GetTenantSettings(ctx context.Context, orgID uuid.UUID) (currency string, taxRate float64, quotePrefix string, err error) {
	var row tenantBusinessSettings
	err = r.db.WithContext(ctx).
		Table("tenant_settings").
		Select("currency, tax_rate, quote_prefix, next_quote_number").
		Where("org_id = ?", orgID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "ARS", 21.0, "PRE", nil
		}
		return "", 0, "", err
	}
	row = normalizeSettings(row)
	return row.Currency, row.TaxRate, row.QuotePrefix, nil
}

type CreateItemInput struct {
	ProductID   *uuid.UUID
	ServiceID   *uuid.UUID
	Description string
	Quantity    float64
	UnitPrice   float64
	TaxRate     float64
	Subtotal    float64
	SortOrder   int
}

type CreateInput struct {
	OrgID        uuid.UUID
	BranchID     *uuid.UUID
	CustomerID   *uuid.UUID
	CustomerName string
	Subtotal     float64
	TaxTotal     float64
	Total        float64
	Currency     string
	IsFavorite   bool
	Tags         []string
	Notes        string
	ValidUntil   *time.Time
	CreatedBy    string
	Tags         []string
	Metadata     map[string]any
	Items        []CreateItemInput
}

func (r *Repository) Create(ctx context.Context, in CreateInput) (quotedomain.Quote, error) {
	var out quotedomain.Quote
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tenant, err := r.getOrCreateTenantSettingsForUpdate(ctx, tx, in.OrgID)
		if err != nil {
			return err
		}

		number := fmt.Sprintf("%s-%05d", tenant.QuotePrefix, tenant.NextQuoteNumber)
		quoteRow := models.QuoteModel{
			ID:           uuid.New(),
			OrgID:        in.OrgID,
			BranchID:     in.BranchID,
			Number:       number,
			CustomerID:   in.CustomerID,
			CustomerName: strings.TrimSpace(in.CustomerName),
			Status:       "draft",
			Subtotal:     in.Subtotal,
			TaxTotal:     in.TaxTotal,
			Total:        in.Total,
			Currency:     coalesce(in.Currency, tenant.Currency),
			IsFavorite:   in.IsFavorite,
			Tags:         pq.StringArray(utils.NormalizeTags(in.Tags)),
			Notes:        strings.TrimSpace(in.Notes),
			ValidUntil:   in.ValidUntil,
			CreatedBy:    strings.TrimSpace(in.CreatedBy),
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
			Tags:         pq.StringArray(utils.NormalizeTags(in.Tags)),
			Metadata:     metadataToJSONBytesQuotes(in.Metadata),
		}
		if err := tx.Create(&quoteRow).Error; err != nil {
			return err
		}

		itemRows := make([]models.QuoteItemModel, 0, len(in.Items))
		for _, item := range in.Items {
			itemRows = append(itemRows, models.QuoteItemModel{
				ID:          uuid.New(),
				QuoteID:     quoteRow.ID,
				ProductID:   item.ProductID,
				ServiceID:   item.ServiceID,
				Description: strings.TrimSpace(item.Description),
				Quantity:    item.Quantity,
				UnitPrice:   item.UnitPrice,
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

		if err := tx.Table("tenant_settings").Where("org_id = ?", in.OrgID).
			Updates(map[string]any{
				"next_quote_number": tenant.NextQuoteNumber + 1,
				"updated_at":        gorm.Expr("now()"),
			}).Error; err != nil {
			return err
		}

		out = quoteToDomain(quoteRow, itemRows)
		return nil
	})
	if err != nil {
		return quotedomain.Quote{}, err
	}
	return out, nil
}

type ListParams struct {
	OrgID      uuid.UUID
	BranchID   *uuid.UUID
	Limit      int
	After      *uuid.UUID
	Status     string
	CustomerID *uuid.UUID
	From       *time.Time
	To         *time.Time
}

func (r *Repository) List(ctx context.Context, p ListParams) ([]quotedomain.Quote, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})

	q := r.db.WithContext(ctx).Model(&models.QuoteModel{}).Where("org_id = ? AND archived_at IS NULL", p.OrgID)
	if p.BranchID != nil && *p.BranchID != uuid.Nil {
		q = q.Where("(branch_id = ? OR branch_id IS NULL)", *p.BranchID)
	}
	if s := strings.TrimSpace(p.Status); s != "" {
		q = q.Where("status = ?", s)
	}
	if p.CustomerID != nil && *p.CustomerID != uuid.Nil {
		q = q.Where("party_id = ?", *p.CustomerID)
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

	var rows []models.QuoteModel
	if err := q.Order("id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	out := make([]quotedomain.Quote, 0, len(rows))
	for _, row := range rows {
		out = append(out, quoteToDomain(row, nil))
	}

	var next *uuid.UUID
	if hasMore && len(rows) > 0 {
		v := rows[len(rows)-1].ID
		next = &v
	}
	return out, total, hasMore, next, nil
}

// ListArchived devuelve presupuestos con archivo lógico (misma convención que clientes/proveedores).
func (r *Repository) ListArchived(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]quotedomain.Quote, error) {
	q := r.db.WithContext(ctx).Where("org_id = ? AND archived_at IS NOT NULL", orgID)
	if branchID != nil && *branchID != uuid.Nil {
		q = q.Where("(branch_id = ? OR branch_id IS NULL)", *branchID)
	}

	var rows []models.QuoteModel
	if err := q.
		Order("updated_at DESC").
		Limit(200).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]quotedomain.Quote, 0, len(rows))
	for _, row := range rows {
		out = append(out, quoteToDomain(row, nil))
	}
	return out, nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, quoteID uuid.UUID) (quotedomain.Quote, error) {
	var quoteRow models.QuoteModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, quoteID).Take(&quoteRow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return quotedomain.Quote{}, gorm.ErrRecordNotFound
		}
		return quotedomain.Quote{}, err
	}

	var itemRows []models.QuoteItemModel
	if err := r.db.WithContext(ctx).Where("quote_id = ?", quoteID).Order("sort_order ASC").Find(&itemRows).Error; err != nil {
		return quotedomain.Quote{}, err
	}
	return quoteToDomain(quoteRow, itemRows), nil
}

type UpdateInput struct {
	OrgID        uuid.UUID
	ID           uuid.UUID
	CustomerID   *uuid.UUID
	CustomerName string
	Subtotal     float64
	TaxTotal     float64
	Total        float64
	Currency     string
	IsFavorite   bool
	Tags         []string
	Notes        string
	ValidUntil   *time.Time
	Tags         []string
	Metadata     map[string]any
	Items        []CreateItemInput
}

func (r *Repository) UpdateDraft(ctx context.Context, in UpdateInput) (quotedomain.Quote, error) {
	var out quotedomain.Quote
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.QuoteModel
		if err := tx.Where("org_id = ? AND id = ?", in.OrgID, in.ID).Take(&existing).Error; err != nil {
			return err
		}
		if existing.Status != "draft" {
			return ErrQuoteNotDraft
		}

		updates := map[string]any{
			"party_id":    in.CustomerID,
			"party_name":  strings.TrimSpace(in.CustomerName),
			"subtotal":    in.Subtotal,
			"tax_total":   in.TaxTotal,
			"total":       in.Total,
			"currency":    strings.TrimSpace(in.Currency),
			"is_favorite": in.IsFavorite,
			"tags":        pq.StringArray(utils.NormalizeTags(in.Tags)),
			"notes":       strings.TrimSpace(in.Notes),
			"valid_until": in.ValidUntil,
			"tags":        pq.StringArray(utils.NormalizeTags(in.Tags)),
			"metadata":    metadataToJSONBytesQuotes(in.Metadata),
			"updated_at":  gorm.Expr("now()"),
		}
		if err := tx.Model(&models.QuoteModel{}).
			Where("org_id = ? AND id = ? AND status = 'draft'", in.OrgID, in.ID).
			Updates(updates).Error; err != nil {
			return err
		}

		if err := tx.Where("quote_id = ?", in.ID).Delete(&models.QuoteItemModel{}).Error; err != nil {
			return err
		}
		itemRows := make([]models.QuoteItemModel, 0, len(in.Items))
		for _, item := range in.Items {
			itemRows = append(itemRows, models.QuoteItemModel{
				ID:          uuid.New(),
				QuoteID:     in.ID,
				ProductID:   item.ProductID,
				ServiceID:   item.ServiceID,
				Description: strings.TrimSpace(item.Description),
				Quantity:    item.Quantity,
				UnitPrice:   item.UnitPrice,
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

		q, err := getByIDWithTx(ctx, tx, in.OrgID, in.ID)
		if err != nil {
			return err
		}
		out = q
		return nil
	})
	if err != nil {
		return quotedomain.Quote{}, err
	}
	return out, nil
}

func (r *Repository) DeleteDraft(ctx context.Context, orgID, quoteID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ? AND status = 'draft'", orgID, quoteID).
		Delete(&models.QuoteModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var exists int64
		if err := r.db.WithContext(ctx).Model(&models.QuoteModel{}).
			Where("org_id = ? AND id = ?", orgID, quoteID).Count(&exists).Error; err != nil {
			return err
		}
		if exists > 0 {
			return ErrQuoteNotDraft
		}
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) Archive(ctx context.Context, orgID, quoteID uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&models.QuoteModel{}).
		Where("org_id = ? AND id = ? AND archived_at IS NULL", orgID, quoteID).
		Updates(map[string]any{"archived_at": gorm.Expr("now()"), "updated_at": gorm.Expr("now()")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) Restore(ctx context.Context, orgID, quoteID uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&models.QuoteModel{}).
		Where("org_id = ? AND id = ? AND archived_at IS NOT NULL", orgID, quoteID).
		Updates(map[string]any{"archived_at": nil, "updated_at": gorm.Expr("now()")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) HardDelete(ctx context.Context, orgID, quoteID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Solo se permite hard delete de presupuestos archivados.
		var count int64
		if err := tx.Model(&models.QuoteModel{}).
			Where("org_id = ? AND id = ? AND archived_at IS NOT NULL", orgID, quoteID).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
		// Eliminar items asociados.
		if err := tx.Where("quote_id = ?", quoteID).Delete(&models.QuoteItemModel{}).Error; err != nil {
			return err
		}
		return tx.Where("org_id = ? AND id = ?", orgID, quoteID).Delete(&models.QuoteModel{}).Error
	})
}

func (r *Repository) SetStatus(ctx context.Context, orgID, quoteID uuid.UUID, status string) (quotedomain.Quote, error) {
	res := r.db.WithContext(ctx).Model(&models.QuoteModel{}).
		Where("org_id = ? AND id = ?", orgID, quoteID).
		Updates(map[string]any{"status": status, "updated_at": gorm.Expr("now()")})
	if res.Error != nil {
		return quotedomain.Quote{}, res.Error
	}
	if res.RowsAffected == 0 {
		return quotedomain.Quote{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, orgID, quoteID)
}

func (r *Repository) getOrCreateTenantSettingsForUpdate(ctx context.Context, tx *gorm.DB, orgID uuid.UUID) (tenantBusinessSettings, error) {
	var tenant tenantBusinessSettings
	err := tx.WithContext(ctx).Table("tenant_settings").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("currency, tax_rate, quote_prefix, next_quote_number").
		Where("org_id = ?", orgID).Take(&tenant).Error
	if err == nil {
		tenant = normalizeSettings(tenant)
		return r.syncNextQuoteNumber(ctx, tx, orgID, tenant)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return tenantBusinessSettings{}, err
	}

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
	if err := tx.WithContext(ctx).Table("tenant_settings").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("currency, tax_rate, quote_prefix, next_quote_number").
		Where("org_id = ?", orgID).Take(&tenant).Error; err != nil {
		return tenantBusinessSettings{}, err
	}
	tenant = normalizeSettings(tenant)
	return r.syncNextQuoteNumber(ctx, tx, orgID, tenant)
}

func (r *Repository) syncNextQuoteNumber(ctx context.Context, tx *gorm.DB, orgID uuid.UUID, tenant tenantBusinessSettings) (tenantBusinessSettings, error) {
	pattern := fmt.Sprintf("%s-%%", tenant.QuotePrefix)
	var maxExisting int
	if err := tx.WithContext(ctx).
		Table("quotes").
		Select("COALESCE(MAX(CAST(right(number, 5) AS INTEGER)), 0)").
		Where("org_id = ? AND number LIKE ?", orgID, pattern).
		Scan(&maxExisting).Error; err != nil {
		return tenantBusinessSettings{}, err
	}
	if tenant.NextQuoteNumber <= maxExisting {
		tenant.NextQuoteNumber = maxExisting + 1
		if err := tx.WithContext(ctx).Table("tenant_settings").
			Where("org_id = ?", orgID).
			Updates(map[string]any{
				"next_quote_number": tenant.NextQuoteNumber,
				"updated_at":        gorm.Expr("now()"),
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
	if strings.TrimSpace(out.QuotePrefix) == "" {
		out.QuotePrefix = "PRE"
	}
	if out.NextQuoteNumber <= 0 {
		out.NextQuoteNumber = 1
	}
	return out
}

func getByIDWithTx(ctx context.Context, tx *gorm.DB, orgID, quoteID uuid.UUID) (quotedomain.Quote, error) {
	var quoteRow models.QuoteModel
	if err := tx.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, quoteID).Take(&quoteRow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return quotedomain.Quote{}, gorm.ErrRecordNotFound
		}
		return quotedomain.Quote{}, err
	}
	var itemRows []models.QuoteItemModel
	if err := tx.WithContext(ctx).Where("quote_id = ?", quoteID).Order("sort_order ASC").Find(&itemRows).Error; err != nil {
		return quotedomain.Quote{}, err
	}
	return quoteToDomain(quoteRow, itemRows), nil
}

func quoteToDomain(quoteRow models.QuoteModel, itemRows []models.QuoteItemModel) quotedomain.Quote {
	items := make([]quotedomain.QuoteItem, 0, len(itemRows))
	for _, row := range itemRows {
		items = append(items, quotedomain.QuoteItem{
			ID:          row.ID,
			QuoteID:     row.QuoteID,
			ProductID:   row.ProductID,
			ServiceID:   row.ServiceID,
			Description: row.Description,
			Quantity:    row.Quantity,
			UnitPrice:   row.UnitPrice,
			TaxRate:     row.TaxRate,
			Subtotal:    row.Subtotal,
			SortOrder:   row.SortOrder,
		})
	}
	return quotedomain.Quote{
		ID:           quoteRow.ID,
		OrgID:        quoteRow.OrgID,
		BranchID:     quoteRow.BranchID,
		Number:       quoteRow.Number,
		CustomerID:   quoteRow.CustomerID,
		CustomerName: quoteRow.CustomerName,
		Status:       quoteRow.Status,
		Items:        items,
		Subtotal:     quoteRow.Subtotal,
		TaxTotal:     quoteRow.TaxTotal,
		Total:        quoteRow.Total,
		Currency:     quoteRow.Currency,
		IsFavorite:   quoteRow.IsFavorite,
		Tags:         append([]string(nil), quoteRow.Tags...),
		Notes:        quoteRow.Notes,
		ValidUntil:   quoteRow.ValidUntil,
		CreatedBy:    quoteRow.CreatedBy,
		CreatedAt:    quoteRow.CreatedAt,
		UpdatedAt:    quoteRow.UpdatedAt,
		ArchivedAt:   quoteRow.ArchivedAt,
		Tags:         append([]string(nil), quoteRow.Tags...),
		Metadata:     metadataFromJSONBytesQuotes(quoteRow.Metadata),
	}
}

func metadataFromJSONBytesQuotes(b []byte) map[string]any {
	if len(b) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		return map[string]any{}
	}
	return m
}

func metadataToJSONBytesQuotes(m map[string]any) []byte {
	if m == nil || len(m) == 0 {
		return []byte("{}")
	}
	b, err := json.Marshal(m)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func mergeMetadataJSONQuotes(current []byte, patch map[string]any) ([]byte, error) {
	base := metadataFromJSONBytesQuotes(current)
	for k, v := range patch {
		if k == "favorite" && !truthyMetadataQuotes(v) {
			delete(base, "favorite")
			continue
		}
		base[k] = v
	}
	return json.Marshal(base)
}

func truthyMetadataQuotes(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		return s == "true" || s == "1"
	case float64:
		return t != 0
	case int:
		return t != 0
	default:
		return v != nil
	}
}

// PatchAnnotations actualiza etiquetas, metadata y campos de texto permitidos fuera del borrador.
func (r *Repository) PatchAnnotations(ctx context.Context, orgID, id uuid.UUID, patch QuotePatchFields) (quotedomain.Quote, error) {
	var row models.QuoteModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		return quotedomain.Quote{}, err
	}
	updates := map[string]any{"updated_at": gorm.Expr("now()")}
	if patch.Tags != nil {
		updates["tags"] = pq.StringArray(utils.NormalizeTags(*patch.Tags))
	}
	if patch.Metadata != nil {
		merged, err := mergeMetadataJSONQuotes(row.Metadata, *patch.Metadata)
		if err != nil {
			return quotedomain.Quote{}, err
		}
		updates["metadata"] = merged
	}
	if patch.Notes != nil {
		updates["notes"] = strings.TrimSpace(*patch.Notes)
	}
	if patch.CustomerName != nil {
		updates["party_name"] = strings.TrimSpace(*patch.CustomerName)
	}
	if len(updates) == 1 {
		return r.GetByID(ctx, orgID, id)
	}
	if err := r.db.WithContext(ctx).Model(&models.QuoteModel{}).Where("org_id = ? AND id = ?", orgID, id).Updates(updates).Error; err != nil {
		return quotedomain.Quote{}, err
	}
	return r.GetByID(ctx, orgID, id)
}

func coalesce(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}
