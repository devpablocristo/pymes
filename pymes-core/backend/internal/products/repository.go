package products

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/products/repository/models"
	productdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/products/usecases/domain"
	"github.com/devpablocristo/pymes/pkgs/go-pkg/utils"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type ListParams struct {
	OrgID  uuid.UUID
	Limit  int
	After  *uuid.UUID
	Search string
	Type   string
	Tag    string
	Sort   string
	Order  string
}

func (r *Repository) List(ctx context.Context, p ListParams) ([]productdomain.Product, int64, bool, *uuid.UUID, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	q := r.db.WithContext(ctx).Model(&models.ProductModel{}).Where("org_id = ? AND deleted_at IS NULL", p.OrgID)
	if t := strings.TrimSpace(p.Type); t != "" {
		q = q.Where("type = ?", t)
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
	row := models.ProductModel{
		ID:          uuid.New(),
		OrgID:       in.OrgID,
		Type:        strings.TrimSpace(in.Type),
		SKU:         strings.TrimSpace(in.SKU),
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		Unit:        strings.TrimSpace(in.Unit),
		Price:       in.Price,
		CostPrice:   in.CostPrice,
		TaxRate:     in.TaxRate,
		TrackStock:  in.TrackStock,
		Tags:        pq.StringArray(utils.NormalizeTags(in.Tags)),
		Metadata:    meta,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return productdomain.Product{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (productdomain.Product, error) {
	var row models.ProductModel
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return productdomain.Product{}, gorm.ErrRecordNotFound
		}
		return productdomain.Product{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in productdomain.Product) (productdomain.Product, error) {
	meta, _ := json.Marshal(in.Metadata)
	updates := map[string]any{
		"type":        strings.TrimSpace(in.Type),
		"sku":         strings.TrimSpace(in.SKU),
		"name":        strings.TrimSpace(in.Name),
		"description": strings.TrimSpace(in.Description),
		"unit":        strings.TrimSpace(in.Unit),
		"price":       in.Price,
		"cost_price":  in.CostPrice,
		"tax_rate":    in.TaxRate,
		"track_stock": in.TrackStock,
		"tags":        pq.StringArray(utils.NormalizeTags(in.Tags)),
		"metadata":    meta,
		"updated_at":  time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&models.ProductModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return productdomain.Product{}, res.Error
	}
	if res.RowsAffected == 0 {
		return productdomain.Product{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func (r *Repository) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&models.ProductModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).
		Updates(map[string]any{"deleted_at": gorm.Expr("now()"), "updated_at": gorm.Expr("now()")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func toDomain(row models.ProductModel) productdomain.Product {
	meta := map[string]any{}
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &meta)
	}
	if meta == nil {
		meta = map[string]any{}
	}
	return productdomain.Product{
		ID:          row.ID,
		OrgID:       row.OrgID,
		Type:        row.Type,
		SKU:         row.SKU,
		Name:        row.Name,
		Description: row.Description,
		Unit:        row.Unit,
		Price:       row.Price,
		CostPrice:   row.CostPrice,
		TaxRate:     row.TaxRate,
		TrackStock:  row.TrackStock,
		Tags:        append([]string(nil), row.Tags...),
		Metadata:    meta,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		DeletedAt:   row.DeletedAt,
	}
}
