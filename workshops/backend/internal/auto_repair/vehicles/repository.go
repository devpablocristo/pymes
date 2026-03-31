package vehicles

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/vehicles/repository/models"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/vehicles/usecases/domain"
)

// ErrVehicleHasWorkOrders indica que no se puede borrar en duro mientras existan OTs.
var ErrVehicleHasWorkOrders = errors.New("vehicle has work orders")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, p ListParams) ([]domain.Vehicle, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&models.VehicleModel{}).Where("org_id = ? AND archived_at IS NULL", p.OrgID)
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

	var rows []models.VehicleModel
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]domain.Vehicle, 0, len(rows))
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

func (r *Repository) ListArchived(ctx context.Context, orgID uuid.UUID) ([]domain.Vehicle, error) {
	var rows []models.VehicleModel
	err := r.db.WithContext(ctx).
		Model(&models.VehicleModel{}).
		Where("org_id = ? AND archived_at IS NOT NULL", orgID).
		Order("updated_at DESC").
		Limit(200).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.Vehicle, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) Create(ctx context.Context, in domain.Vehicle) (domain.Vehicle, error) {
	row := models.VehicleModel{
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
		return domain.Vehicle{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Vehicle, error) {
	var row models.VehicleModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Vehicle{}, gorm.ErrRecordNotFound
		}
		return domain.Vehicle{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in domain.Vehicle) (domain.Vehicle, error) {
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
	res := r.db.WithContext(ctx).Model(&models.VehicleModel{}).
		Where("org_id = ? AND id = ? AND archived_at IS NULL", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return domain.Vehicle{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.Vehicle{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func (r *Repository) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&models.VehicleModel{}).
		Where("org_id = ? AND id = ? AND archived_at IS NULL", orgID, id).
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
	res := r.db.WithContext(ctx).Model(&models.VehicleModel{}).
		Where("org_id = ? AND id = ? AND archived_at IS NOT NULL", orgID, id).
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
	if err := r.db.WithContext(ctx).Table("workshops.work_orders").
		Where("org_id = ? AND vehicle_id = ?", orgID, id).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrVehicleHasWorkOrders
	}
	var row models.VehicleModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		return err
	}
	if row.ArchivedAt == nil {
		return gorm.ErrRecordNotFound
	}
	res := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Delete(&models.VehicleModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func toDomain(row models.VehicleModel) domain.Vehicle {
	return domain.Vehicle{
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
		ArchivedAt:   row.ArchivedAt,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}
