package vehicles

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pkgs/go-pkg/pagination"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, p ListParams) ([]Vehicle, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, 20, 100)
	q := r.db.WithContext(ctx).Model(&VehicleModel{}).Where("org_id = ?", p.OrgID)
	if search := strings.TrimSpace(p.Search); search != "" {
		like := "%" + search + "%"
		q = q.Where("(license_plate ILIKE ? OR make ILIKE ? OR model ILIKE ? OR customer_name ILIKE ? OR vin ILIKE ?)", like, like, like, like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}
	if p.After != nil && *p.After != uuid.Nil {
		q = q.Where("id < ?", *p.After)
	}
	q = q.Order("id DESC")

	var rows []VehicleModel
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]Vehicle, 0, len(rows))
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

func (r *Repository) Create(ctx context.Context, in Vehicle) (Vehicle, error) {
	row := VehicleModel{
		ID:           uuid.New(),
		OrgID:        in.OrgID,
		CustomerID:   in.CustomerID,
		CustomerName: in.CustomerName,
		LicensePlate: in.LicensePlate,
		VIN:          in.VIN,
		Make:         in.Make,
		Model:        in.Model,
		Year:         in.Year,
		Kilometers:   in.Kilometers,
		Color:        in.Color,
		Notes:        in.Notes,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return Vehicle{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (Vehicle, error) {
	var row VehicleModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Vehicle{}, gorm.ErrRecordNotFound
		}
		return Vehicle{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in Vehicle) (Vehicle, error) {
	updates := map[string]any{
		"customer_id":   in.CustomerID,
		"customer_name": in.CustomerName,
		"license_plate": in.LicensePlate,
		"vin":           in.VIN,
		"make":          in.Make,
		"model":         in.Model,
		"year":          in.Year,
		"kilometers":    in.Kilometers,
		"color":         in.Color,
		"notes":         in.Notes,
		"updated_at":    time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&VehicleModel{}).
		Where("org_id = ? AND id = ?", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return Vehicle{}, res.Error
	}
	if res.RowsAffected == 0 {
		return Vehicle{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func toDomain(row VehicleModel) Vehicle {
	return Vehicle{
		ID:           row.ID,
		OrgID:        row.OrgID,
		CustomerID:   row.CustomerID,
		CustomerName: row.CustomerName,
		LicensePlate: row.LicensePlate,
		VIN:          row.VIN,
		Make:         row.Make,
		Model:        row.Model,
		Year:         row.Year,
		Kilometers:   row.Kilometers,
		Color:        row.Color,
		Notes:        row.Notes,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}
