package bicycles

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/backend/go/pagination"
	"github.com/devpablocristo/pymes/workshops/backend/internal/bike_shop/bicycles/repository/models"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/bike_shop/bicycles/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, p ListParams) ([]domain.Bicycle, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&models.BicycleModel{}).Where("org_id = ?", p.OrgID)
	if search := strings.TrimSpace(p.Search); search != "" {
		like := "%" + search + "%"
		q = q.Where("(frame_number ILIKE ? OR make ILIKE ? OR model ILIKE ? OR customer_name ILIKE ? OR bike_type ILIKE ?)", like, like, like, like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}
	if p.After != nil && *p.After != uuid.Nil {
		q = q.Where("id < ?", *p.After)
	}
	q = q.Order("id DESC")

	var rows []models.BicycleModel
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]domain.Bicycle, 0, len(rows))
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

func (r *Repository) Create(ctx context.Context, in domain.Bicycle) (domain.Bicycle, error) {
	row := models.BicycleModel{
		ID:              uuid.New(),
		OrgID:           in.OrgID,
		CustomerID:      in.CustomerID,
		CustomerName:    in.CustomerName,
		FrameNumber:     in.FrameNumber,
		Make:            in.Make,
		Model:           in.Model,
		BikeType:        in.BikeType,
		Size:            in.Size,
		WheelSizeInches: in.WheelSizeInches,
		Color:           in.Color,
		EbikeNotes:      in.EbikeNotes,
		Notes:           in.Notes,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Bicycle{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Bicycle, error) {
	var row models.BicycleModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Bicycle{}, gorm.ErrRecordNotFound
		}
		return domain.Bicycle{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in domain.Bicycle) (domain.Bicycle, error) {
	updates := map[string]any{
		"customer_id":       in.CustomerID,
		"customer_name":     in.CustomerName,
		"frame_number":      in.FrameNumber,
		"make":              in.Make,
		"model":             in.Model,
		"bike_type":         in.BikeType,
		"size":              in.Size,
		"wheel_size_inches": in.WheelSizeInches,
		"color":             in.Color,
		"ebike_notes":       in.EbikeNotes,
		"notes":             in.Notes,
		"updated_at":        time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&models.BicycleModel{}).
		Where("org_id = ? AND id = ?", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return domain.Bicycle{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.Bicycle{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func toDomain(row models.BicycleModel) domain.Bicycle {
	return domain.Bicycle{
		ID:              row.ID,
		OrgID:           row.OrgID,
		CustomerID:      row.CustomerID,
		CustomerName:    row.CustomerName,
		FrameNumber:     row.FrameNumber,
		Make:            row.Make,
		Model:           row.Model,
		BikeType:        row.BikeType,
		Size:            row.Size,
		WheelSizeInches: row.WheelSizeInches,
		Color:           row.Color,
		EbikeNotes:      row.EbikeNotes,
		Notes:           row.Notes,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}
