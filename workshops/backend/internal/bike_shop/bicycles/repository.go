package bicycles

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/http/go/pagination"
	utils "github.com/devpablocristo/core/validate/go/stringutil"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/bike_shop/bicycles/usecases/domain"
	assetmodels "github.com/devpablocristo/pymes/workshops/backend/internal/customerassets/repository/models"
)

const bicycleAssetType = "bicycle"

var ErrBicycleHasWorkOrders = errors.New("bicycle has work orders")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, p ListParams) ([]domain.Bicycle, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&assetmodels.CustomerAssetModel{}).
		Where("tenant_id = ? AND asset_type = ? AND archived_at IS NULL", p.TenantID, bicycleAssetType)
	if search := strings.TrimSpace(p.Search); search != "" {
		like := "%" + search + "%"
		q = q.Where("(serial_number ILIKE ? OR brand ILIKE ? OR model ILIKE ? OR customer_name ILIKE ? OR metadata->>'bike_type' ILIKE ?)", like, like, like, like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}
	if p.After != nil && *p.After != uuid.Nil {
		q = q.Where("id < ?", *p.After)
	}
	q = q.Order("id DESC")

	var rows []assetmodels.CustomerAssetModel
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

func (r *Repository) ListArchived(ctx context.Context, tenantID uuid.UUID) ([]domain.Bicycle, error) {
	var rows []assetmodels.CustomerAssetModel
	err := r.db.WithContext(ctx).
		Model(&assetmodels.CustomerAssetModel{}).
		Where("tenant_id = ? AND asset_type = ? AND archived_at IS NOT NULL", tenantID, bicycleAssetType).
		Order("updated_at DESC").
		Limit(200).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.Bicycle, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) Create(ctx context.Context, in domain.Bicycle) (domain.Bicycle, error) {
	now := time.Now().UTC()
	row := toAssetModel(in)
	row.ID = uuid.New()
	row.CreatedAt = now
	row.UpdatedAt = now
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Bicycle{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (domain.Bicycle, error) {
	var row assetmodels.CustomerAssetModel
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND asset_type = ? AND id = ?", tenantID, bicycleAssetType, id).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Bicycle{}, gorm.ErrRecordNotFound
		}
		return domain.Bicycle{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in domain.Bicycle) (domain.Bicycle, error) {
	row := toAssetModel(in)
	updates := map[string]any{
		"customer_id":   row.CustomerID,
		"customer_name": row.CustomerName,
		"label":         row.Label,
		"brand":         row.Brand,
		"model":         row.Model,
		"serial_number": row.SerialNumber,
		"color":         row.Color,
		"notes":         row.Notes,
		"metadata":      row.Metadata,
		"is_favorite":   row.IsFavorite,
		"tags":          row.Tags,
		"updated_at":    time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&assetmodels.CustomerAssetModel{}).
		Where("tenant_id = ? AND asset_type = ? AND id = ? AND archived_at IS NULL", in.TenantID, bicycleAssetType, in.ID).
		Updates(updates)
	if res.Error != nil {
		return domain.Bicycle{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.Bicycle{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.TenantID, in.ID)
}

func (r *Repository) SoftDelete(ctx context.Context, tenantID, id uuid.UUID) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&assetmodels.CustomerAssetModel{}).
		Where("tenant_id = ? AND asset_type = ? AND id = ? AND archived_at IS NULL", tenantID, bicycleAssetType, id).
		Updates(map[string]any{"archived_at": now, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) Restore(ctx context.Context, tenantID, id uuid.UUID) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&assetmodels.CustomerAssetModel{}).
		Where("tenant_id = ? AND asset_type = ? AND id = ? AND archived_at IS NOT NULL", tenantID, bicycleAssetType, id).
		Updates(map[string]any{"archived_at": nil, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) HardDelete(ctx context.Context, tenantID, id uuid.UUID) error {
	var count int64
	if err := r.db.WithContext(ctx).Table("workshops.work_orders").
		Where("tenant_id = ? AND asset_type = ? AND asset_id = ?", tenantID, bicycleAssetType, id).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrBicycleHasWorkOrders
	}
	var row assetmodels.CustomerAssetModel
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND asset_type = ? AND id = ?", tenantID, bicycleAssetType, id).
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
		Where("tenant_id = ? AND asset_type = ? AND id = ?", tenantID, bicycleAssetType, id).
		Delete(&assetmodels.CustomerAssetModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func toAssetModel(in domain.Bicycle) assetmodels.CustomerAssetModel {
	metadata := map[string]any{
		"frame_number":      in.FrameNumber,
		"bike_type":         in.BikeType,
		"size":              in.Size,
		"wheel_size_inches": in.WheelSizeInches,
		"ebike_notes":       in.EbikeNotes,
	}
	metadataJSON, _ := json.Marshal(metadata)
	return assetmodels.CustomerAssetModel{
		ID:           in.ID,
		TenantID:     in.TenantID,
		AssetType:    bicycleAssetType,
		CustomerID:   in.CustomerID,
		CustomerName: in.CustomerName,
		Label:        in.DisplayLabel(),
		Brand:        in.Brand,
		Model:        in.Model,
		SerialNumber: in.FrameNumber,
		Color:        in.Color,
		Notes:        in.Notes,
		Metadata:     metadataJSON,
		IsFavorite:   in.IsFavorite,
		Tags:         pq.StringArray(utils.NormalizeTags(in.Tags)),
		ArchivedAt:   in.ArchivedAt,
		CreatedAt:    in.CreatedAt,
		UpdatedAt:    in.UpdatedAt,
	}
}

func toDomain(row assetmodels.CustomerAssetModel) domain.Bicycle {
	metadata := map[string]any{}
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &metadata)
	}
	return domain.Bicycle{
		ID:              row.ID,
		TenantID:        row.TenantID,
		CustomerID:      row.CustomerID,
		CustomerName:    row.CustomerName,
		FrameNumber:     stringFromMetadata(metadata, "frame_number", row.SerialNumber),
		Brand:           row.Brand,
		Model:           row.Model,
		BikeType:        stringFromMetadata(metadata, "bike_type", ""),
		Size:            stringFromMetadata(metadata, "size", ""),
		WheelSizeInches: intFromMetadata(metadata, "wheel_size_inches"),
		Color:           row.Color,
		EbikeNotes:      stringFromMetadata(metadata, "ebike_notes", ""),
		Notes:           row.Notes,
		IsFavorite:      row.IsFavorite,
		Tags:            append([]string(nil), row.Tags...),
		ArchivedAt:      row.ArchivedAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func stringFromMetadata(metadata map[string]any, key string, fallback string) string {
	if value, ok := metadata[key].(string); ok {
		return value
	}
	return fallback
}

func intFromMetadata(metadata map[string]any, key string) int {
	switch value := metadata[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}
