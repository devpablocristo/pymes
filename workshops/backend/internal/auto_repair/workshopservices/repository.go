package workshopservices

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/utils/go/pagination"
	"github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/workshopservices/repository/models"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/workshopservices/usecases/domain"
	workshopshared "github.com/devpablocristo/pymes/workshops/backend/internal/shared/workshops"
)

// ErrServiceReferencedInWorkOrders no se puede borrar en duro si hay ítems de OT que lo referencian.
var ErrServiceReferencedInWorkOrders = errors.New("service referenced in work orders")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, p ListParams) ([]domain.Service, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&models.ServiceModel{}).
		Where("org_id = ? AND segment = ? AND archived_at IS NULL", p.OrgID, workshopshared.SegmentAutoRepair)
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

	var rows []models.ServiceModel
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]domain.Service, 0, len(rows))
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

func (r *Repository) ListArchived(ctx context.Context, orgID uuid.UUID) ([]domain.Service, error) {
	var rows []models.ServiceModel
	err := r.db.WithContext(ctx).
		Model(&models.ServiceModel{}).
		Where("org_id = ? AND segment = ? AND archived_at IS NOT NULL", orgID, workshopshared.SegmentAutoRepair).
		Order("updated_at DESC").
		Limit(200).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.Service, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) Create(ctx context.Context, in domain.Service) (domain.Service, error) {
	row := models.ServiceModel{
		ID:              uuid.New(),
		OrgID:           in.OrgID,
		Segment:         workshopshared.SegmentAutoRepair,
		Code:            in.Code,
		Name:            in.Name,
		Description:     in.Description,
		Category:        in.Category,
		EstimatedHours:  in.EstimatedHours,
		BasePrice:       in.BasePrice,
		Currency:        in.Currency,
		TaxRate:         in.TaxRate,
		LinkedProductID: in.LinkedProductID,
		IsActive:        in.IsActive,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Service{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Service, error) {
	var row models.ServiceModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ? AND segment = ?", orgID, id, workshopshared.SegmentAutoRepair).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Service{}, gorm.ErrRecordNotFound
		}
		return domain.Service{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in domain.Service) (domain.Service, error) {
	updates := map[string]any{
		"code":              in.Code,
		"name":              in.Name,
		"description":       in.Description,
		"category":          in.Category,
		"estimated_hours":   in.EstimatedHours,
		"base_price":        in.BasePrice,
		"currency":          in.Currency,
		"tax_rate":          in.TaxRate,
		"linked_product_id": in.LinkedProductID,
		"is_active":         in.IsActive,
		"updated_at":        time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&models.ServiceModel{}).
		Where("org_id = ? AND id = ? AND segment = ? AND archived_at IS NULL", in.OrgID, in.ID, workshopshared.SegmentAutoRepair).
		Updates(updates)
	if res.Error != nil {
		return domain.Service{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.Service{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func (r *Repository) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&models.ServiceModel{}).
		Where("org_id = ? AND id = ? AND segment = ? AND archived_at IS NULL", orgID, id, workshopshared.SegmentAutoRepair).
		Updates(map[string]any{"archived_at": now, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) Restore(ctx context.Context, orgID, id uuid.UUID) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&models.ServiceModel{}).
		Where("org_id = ? AND id = ? AND segment = ? AND archived_at IS NOT NULL", orgID, id, workshopshared.SegmentAutoRepair).
		Updates(map[string]any{"archived_at": nil, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) HardDelete(ctx context.Context, orgID, id uuid.UUID) error {
	var count int64
	if err := r.db.WithContext(ctx).Table("workshops.work_order_items").
		Where("org_id = ? AND service_id = ?", orgID, id).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrServiceReferencedInWorkOrders
	}
	var row models.ServiceModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ? AND segment = ?", orgID, id, workshopshared.SegmentAutoRepair).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		return err
	}
	if row.ArchivedAt == nil {
		return gorm.ErrRecordNotFound
	}
	res := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ? AND segment = ?", orgID, id, workshopshared.SegmentAutoRepair).
		Delete(&models.ServiceModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func toDomain(row models.ServiceModel) domain.Service {
	return domain.Service{
		ID:              row.ID,
		OrgID:           row.OrgID,
		Code:            row.Code,
		Name:            row.Name,
		Description:     row.Description,
		Category:        row.Category,
		EstimatedHours:  row.EstimatedHours,
		BasePrice:       row.BasePrice,
		Currency:        row.Currency,
		TaxRate:         row.TaxRate,
		LinkedProductID: row.LinkedProductID,
		IsActive:        row.IsActive,
		ArchivedAt:      row.ArchivedAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}
