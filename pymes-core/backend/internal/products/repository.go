package products

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	utils "github.com/devpablocristo/core/validate/go/stringutil"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/products/repository/models"
	productdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/products/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

var (
	ErrNotFound      = errors.New("product not found")
	ErrAlreadyExists = errors.New("product already exists")
)

type ListParams struct {
	OrgID    uuid.UUID
	Limit    int
	After    *uuid.UUID
	Search   string
	Tag      string
	Sort     string
	Order    string
	Archived bool
}

func (r *Repository) List(ctx context.Context, p ListParams) ([]productdomain.Product, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&models.ProductModel{}).Where("org_id = ? AND type = 'product'", p.OrgID)
	if p.Archived {
		q = q.Where("deleted_at IS NOT NULL")
	} else {
		q = q.Where("deleted_at IS NULL")
	}
	if tag := strings.TrimSpace(p.Tag); tag != "" {
		q = q.Where("? = ANY(tags)", tag)
	}
	if s := strings.TrimSpace(p.Search); s != "" {
		like := "%" + s + "%"
		q = q.Where("(name ILIKE ? OR description ILIKE ? OR sku ILIKE ?)", like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}
	order := "desc"
	if strings.EqualFold(strings.TrimSpace(p.Order), "asc") {
		order = "asc"
	}
	if p.After != nil && *p.After != uuid.Nil {
		if order == "asc" {
			q = q.Where("id > ?", *p.After)
		} else {
			q = q.Where("id < ?", *p.After)
		}
	}
	q = q.Order("id " + order)
	var rows []models.ProductModel
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]productdomain.Product, 0, len(rows))
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

func (r *Repository) Create(ctx context.Context, in productdomain.Product) (productdomain.Product, error) {
	meta, _ := json.Marshal(in.Metadata)
	imgURLs := append([]string(nil), in.ImageURLs...)
	if imgURLs == nil {
		imgURLs = []string{}
	}
	row := models.ProductModel{
		ID:          uuid.New(),
		OrgID:       in.OrgID,
		Type:        "product",
		SKU:         strings.TrimSpace(in.SKU),
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		Unit:        strings.TrimSpace(in.Unit),
		Price:       in.Price,
		Currency:    strings.TrimSpace(in.Currency),
		CostPrice:   in.CostPrice,
		TaxRate:     in.TaxRate,
		ImageURL:    strings.TrimSpace(in.ImageURL),
		ImageURLs:   pq.StringArray(imgURLs),
		TrackStock:  in.TrackStock,
		IsActive:    in.IsActive,
		IsFavorite:  in.IsFavorite,
		Tags:        pq.StringArray(utils.NormalizeTags(in.Tags)),
		Metadata:    meta,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		if httperrors.IsUniqueViolation(err) {
			return productdomain.Product{}, ErrAlreadyExists
		}
		return productdomain.Product{}, err
	}
	return toDomain(row), nil
}

// GetByID devuelve el producto independientemente de su estado de archivado.
// El caller decide qué hacer con DeletedAt (Update rechaza archivados; el handler los expone con deleted_at).
func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (productdomain.Product, error) {
	var row models.ProductModel
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ? AND type = 'product'", orgID, id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return productdomain.Product{}, ErrNotFound
		}
		return productdomain.Product{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in productdomain.Product) (productdomain.Product, error) {
	meta, _ := json.Marshal(in.Metadata)
	imgURLs := append([]string(nil), in.ImageURLs...)
	if imgURLs == nil {
		imgURLs = []string{}
	}
	updates := map[string]any{
		"type":           "product",
		"sku":            strings.TrimSpace(in.SKU),
		"name":           strings.TrimSpace(in.Name),
		"description":    strings.TrimSpace(in.Description),
		"unit":           strings.TrimSpace(in.Unit),
		"price":          in.Price,
		"price_currency": strings.TrimSpace(in.Currency),
		"cost_price":     in.CostPrice,
		"tax_rate":       in.TaxRate,
		"image_url":      strings.TrimSpace(in.ImageURL),
		"image_urls":     pq.StringArray(imgURLs),
		"track_stock":    in.TrackStock,
		"is_active":      in.IsActive,
		"is_favorite":    in.IsFavorite,
		"tags":           pq.StringArray(utils.NormalizeTags(in.Tags)),
		"metadata":       meta,
		"updated_at":     time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&models.ProductModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL AND type = 'product'", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		if httperrors.IsUniqueViolation(res.Error) {
			return productdomain.Product{}, ErrAlreadyExists
		}
		return productdomain.Product{}, res.Error
	}
	if res.RowsAffected == 0 {
		return productdomain.Product{}, ErrNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func (r *Repository) Archive(ctx context.Context, orgID, id uuid.UUID) error {
	state, err := r.lookupState(ctx, orgID, id)
	if err != nil {
		return err
	}
	if state.DeletedAt != nil {
		return nil
	}
	res := r.db.WithContext(ctx).Model(&models.ProductModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL AND type = 'product'", orgID, id).
		Updates(map[string]any{"deleted_at": gorm.Expr("now()"), "updated_at": gorm.Expr("now()")})
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *Repository) Restore(ctx context.Context, orgID, id uuid.UUID) error {
	state, err := r.lookupState(ctx, orgID, id)
	if err != nil {
		return err
	}
	if state.DeletedAt == nil {
		return nil
	}
	res := r.db.WithContext(ctx).Model(&models.ProductModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NOT NULL AND type = 'product'", orgID, id).
		Updates(map[string]any{"deleted_at": nil, "updated_at": gorm.Expr("now()")})
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	// Borrado físico solo si está archivado (deleted_at). Antes: liberar FKs que no tienen ON DELETE CASCADE.
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"DELETE FROM stock_movements WHERE org_id = ? AND product_id = ?",
			orgID, id,
		).Error; err != nil {
			return err
		}
		updates := []string{
			`UPDATE quote_items SET product_id = NULL WHERE product_id = ? AND EXISTS (SELECT 1 FROM quotes q WHERE q.id = quote_items.quote_id AND q.org_id = ?)`,
			`UPDATE sale_items SET product_id = NULL WHERE product_id = ? AND EXISTS (SELECT 1 FROM sales s WHERE s.id = sale_items.sale_id AND s.org_id = ?)`,
			`UPDATE purchase_items SET product_id = NULL WHERE product_id = ? AND EXISTS (SELECT 1 FROM purchases p WHERE p.id = purchase_items.purchase_id AND p.org_id = ?)`,
			`UPDATE return_items SET product_id = NULL WHERE product_id = ? AND EXISTS (SELECT 1 FROM returns r WHERE r.id = return_items.return_id AND r.org_id = ?)`,
		}
		for _, stmt := range updates {
			if err := tx.Exec(stmt, id, orgID).Error; err != nil {
				return err
			}
		}
		res := tx.Unscoped().
			Where("org_id = ? AND id = ? AND type = 'product' AND deleted_at IS NOT NULL", orgID, id).
			Delete(&models.ProductModel{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (r *Repository) lookupState(ctx context.Context, orgID, id uuid.UUID) (models.ProductModel, error) {
	var row models.ProductModel
	err := r.db.WithContext(ctx).
		Select("id, deleted_at").
		Where("org_id = ? AND id = ? AND type = 'product'", orgID, id).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.ProductModel{}, ErrNotFound
		}
		return models.ProductModel{}, err
	}
	return row, nil
}

func toDomain(row models.ProductModel) productdomain.Product {
	meta := map[string]any{}
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &meta)
	}
	if meta == nil {
		meta = map[string]any{}
	}
	p := productdomain.Product{
		ID:          row.ID,
		OrgID:       row.OrgID,
		SKU:         row.SKU,
		Name:        row.Name,
		Description: row.Description,
		Unit:        row.Unit,
		Price:       row.Price,
		Currency:    row.Currency,
		CostPrice:   row.CostPrice,
		TaxRate:     row.TaxRate,
		ImageURL:    row.ImageURL,
		ImageURLs:   append([]string(nil), row.ImageURLs...),
		TrackStock:  row.TrackStock,
		IsActive:    row.IsActive,
		IsFavorite:  row.IsFavorite,
		Tags:        append([]string(nil), row.Tags...),
		Metadata:    meta,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		DeletedAt:   row.DeletedAt,
	}
	// Reconciliar filas antiguas: metadata.image_urls poblado pero columna image_urls vacía.
	if len(p.ImageURLs) == 0 {
		raw, ok := parseImageURLsFromMetadata(p.Metadata)
		if ok {
			if urls, err := normalizeProductImageURLs(raw); err == nil && len(urls) > 0 {
				p.ImageURLs = urls
				if strings.TrimSpace(p.ImageURL) == "" {
					p.ImageURL = urls[0]
				}
			}
		}
	}
	return p
}
