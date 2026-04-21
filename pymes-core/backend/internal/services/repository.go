package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	utils "github.com/devpablocristo/core/validate/go/stringutil"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/services/repository/models"
	servicedomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/services/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

var (
	ErrNotFound      = errors.New("service not found")
	ErrAlreadyExists = errors.New("service already exists")
	ErrArchived      = errors.New("service archived")
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

func (r *Repository) List(ctx context.Context, p ListParams) ([]servicedomain.Service, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&models.ServiceModel{}).Where("org_id = ?", p.OrgID)
	if !p.Archived {
		q = q.Where("deleted_at IS NULL")
	}
	if tag := strings.TrimSpace(p.Tag); tag != "" {
		q = q.Where("? = ANY(tags)", tag)
	}
	if s := strings.TrimSpace(p.Search); s != "" {
		like := "%" + s + "%"
		q = q.Where("(name ILIKE ? OR description ILIKE ? OR code ILIKE ?)", like, like, like)
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

	var rows []models.ServiceModel
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	out := make([]servicedomain.Service, 0, len(rows))
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

func (r *Repository) Create(ctx context.Context, in servicedomain.Service) (servicedomain.Service, error) {
	meta, _ := json.Marshal(in.Metadata)
	row := models.ServiceModel{
		ID:                     uuid.New(),
		OrgID:                  in.OrgID,
		Code:                   strings.TrimSpace(in.Code),
		Name:                   strings.TrimSpace(in.Name),
		Description:            strings.TrimSpace(in.Description),
		CategoryCode:           strings.TrimSpace(in.CategoryCode),
		SalePrice:              in.SalePrice,
		CostPrice:              in.CostPrice,
		TaxRate:                in.TaxRate,
		Currency:               strings.TrimSpace(in.Currency),
		DefaultDurationMinutes: in.DefaultDurationMinutes,
		IsActive:               in.IsActive,
		IsFavorite:             in.IsFavorite,
		Tags:                   pq.StringArray(utils.NormalizeTags(in.Tags)),
		Metadata:               meta,
		CreatedAt:              time.Now().UTC(),
		UpdatedAt:              time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		if httperrors.IsUniqueViolation(err) {
			return servicedomain.Service{}, ErrAlreadyExists
		}
		return servicedomain.Service{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (servicedomain.Service, error) {
	var row models.ServiceModel
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return servicedomain.Service{}, ErrNotFound
		}
		return servicedomain.Service{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in servicedomain.Service) (servicedomain.Service, error) {
	meta, _ := json.Marshal(in.Metadata)
	updates := map[string]any{
		"code":                     strings.TrimSpace(in.Code),
		"name":                     strings.TrimSpace(in.Name),
		"description":              strings.TrimSpace(in.Description),
		"category_code":            strings.TrimSpace(in.CategoryCode),
		"sale_price":               in.SalePrice,
		"cost_price":               in.CostPrice,
		"tax_rate":                 in.TaxRate,
		"currency":                 strings.TrimSpace(in.Currency),
		"default_duration_minutes": in.DefaultDurationMinutes,
		"is_active":                in.IsActive,
		"is_favorite":              in.IsFavorite,
		"tags":                     pq.StringArray(utils.NormalizeTags(in.Tags)),
		"metadata":                 meta,
		"updated_at":               time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&models.ServiceModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		if httperrors.IsUniqueViolation(res.Error) {
			return servicedomain.Service{}, ErrAlreadyExists
		}
		return servicedomain.Service{}, res.Error
	}
	if res.RowsAffected == 0 {
		return servicedomain.Service{}, ErrNotFound
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
	res := r.db.WithContext(ctx).Model(&models.ServiceModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).
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
	res := r.db.WithContext(ctx).Model(&models.ServiceModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NOT NULL", orgID, id).
		Updates(map[string]any{"deleted_at": nil, "updated_at": gorm.Expr("now()")})
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ?", orgID, id).
		Delete(&models.ServiceModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) lookupState(ctx context.Context, orgID, id uuid.UUID) (models.ServiceModel, error) {
	var row models.ServiceModel
	err := r.db.WithContext(ctx).
		Select("id, deleted_at").
		Where("org_id = ? AND id = ?", orgID, id).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.ServiceModel{}, ErrNotFound
		}
		return models.ServiceModel{}, err
	}
	return row, nil
}

func toDomain(row models.ServiceModel) servicedomain.Service {
	meta := map[string]any{}
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &meta)
	}
	if meta == nil {
		meta = map[string]any{}
	}
	return servicedomain.Service{
		ID:                     row.ID,
		OrgID:                  row.OrgID,
		Code:                   row.Code,
		Name:                   row.Name,
		Description:            row.Description,
		CategoryCode:           row.CategoryCode,
		SalePrice:              row.SalePrice,
		CostPrice:              row.CostPrice,
		TaxRate:                row.TaxRate,
		Currency:               row.Currency,
		DefaultDurationMinutes: row.DefaultDurationMinutes,
		IsActive:               row.IsActive,
		IsFavorite:             row.IsFavorite,
		Tags:                   append([]string(nil), row.Tags...),
		Metadata:               meta,
		CreatedAt:              row.CreatedAt,
		UpdatedAt:              row.UpdatedAt,
		DeletedAt:              row.DeletedAt,
	}
}
