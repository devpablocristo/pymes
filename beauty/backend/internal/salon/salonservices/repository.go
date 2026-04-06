package salonservices

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/devpablocristo/pymes/beauty/backend/internal/salon/salonservices/repository/models"
	domain "github.com/devpablocristo/pymes/beauty/backend/internal/salon/salonservices/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, p ListParams) ([]domain.SalonService, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&models.SalonServiceModel{}).Where("org_id = ?", p.OrgID)
	if search := strings.TrimSpace(p.Search); search != "" {
		like := "%" + search + "%"
		q = q.Where("(code ILIKE ? OR name ILIKE ? OR description ILIKE ? OR category ILIKE ?)", like, like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}
	if p.After != nil && *p.After != uuid.Nil {
		q = q.Where("id < ?", *p.After)
	}
	q = q.Order("id DESC")

	var rows []models.SalonServiceModel
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]domain.SalonService, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	var next *uuid.UUID
	if hasMore && len(rows) > 0 {
		value := rows[len(rows)-1].ID
		next = &value
	}
	return out, total, hasMore, next, nil
}

func (r *Repository) Create(ctx context.Context, in domain.SalonService) (domain.SalonService, error) {
	row := models.SalonServiceModel{
		ID:              uuid.New(),
		OrgID:           in.OrgID,
		Code:            in.Code,
		Name:            in.Name,
		Description:     in.Description,
		Category:        in.Category,
		DurationMinutes: in.DurationMinutes,
		BasePrice:       in.BasePrice,
		Currency:        in.Currency,
		TaxRate:         in.TaxRate,
		LinkedServiceID: in.LinkedServiceID,
		IsActive:        in.IsActive,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.SalonService{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.SalonService, error) {
	var row models.SalonServiceModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.SalonService{}, gorm.ErrRecordNotFound
		}
		return domain.SalonService{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in domain.SalonService) (domain.SalonService, error) {
	updates := map[string]any{
		"code":              in.Code,
		"name":              in.Name,
		"description":       in.Description,
		"category":          in.Category,
		"duration_minutes":  in.DurationMinutes,
		"base_price":        in.BasePrice,
		"currency":          in.Currency,
		"tax_rate":          in.TaxRate,
		"linked_service_id": in.LinkedServiceID,
		"is_active":         in.IsActive,
		"updated_at":        time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&models.SalonServiceModel{}).
		Where("org_id = ? AND id = ?", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return domain.SalonService{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.SalonService{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func toDomain(row models.SalonServiceModel) domain.SalonService {
	return domain.SalonService{
		ID:              row.ID,
		OrgID:           row.OrgID,
		Code:            row.Code,
		Name:            row.Name,
		Description:     row.Description,
		Category:        row.Category,
		DurationMinutes: row.DurationMinutes,
		BasePrice:       row.BasePrice,
		Currency:        row.Currency,
		TaxRate:         row.TaxRate,
		LinkedServiceID: row.LinkedServiceID,
		IsActive:        row.IsActive,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}
