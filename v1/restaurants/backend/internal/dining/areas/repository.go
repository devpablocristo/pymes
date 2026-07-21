package areas

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/devpablocristo/platform/http/go/pagination"
	utils "github.com/devpablocristo/platform/validate/go/stringutil"
	"github.com/devpablocristo/pymes/restaurants/backend/internal/dining/areas/repository/models"
	domain "github.com/devpablocristo/pymes/restaurants/backend/internal/dining/areas/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ExistsForOrg(ctx context.Context, orgID, areaID uuid.UUID) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.DiningAreaModel{}).
		Where("org_id = ? AND id = ?", orgID, areaID).
		Limit(1).
		Count(&n).Error
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *Repository) List(ctx context.Context, p ListParams) ([]domain.DiningArea, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&models.DiningAreaModel{}).Where("org_id = ?", p.OrgID)
	if p.Archived {
		q = q.Where("deleted_at IS NOT NULL")
	} else {
		q = q.Where("deleted_at IS NULL")
	}
	if search := strings.TrimSpace(p.Search); search != "" {
		like := "%" + search + "%"
		q = q.Where("name ILIKE ?", like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}
	if p.After != nil && *p.After != uuid.Nil {
		q = q.Where("id < ?", *p.After)
	}
	q = q.Order("sort_order ASC, id DESC")

	var rows []models.DiningAreaModel
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]domain.DiningArea, 0, len(rows))
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

func (r *Repository) Create(ctx context.Context, in domain.DiningArea) (domain.DiningArea, error) {
	md := in.Metadata
	if md == nil {
		md = map[string]any{}
	}
	meta, _ := json.Marshal(md)
	row := models.DiningAreaModel{
		ID:         uuid.New(),
		OrgID:   in.OrgID,
		Name:       in.Name,
		SortOrder:  in.SortOrder,
		IsFavorite: in.IsFavorite,
		Tags:       pq.StringArray(utils.NormalizeTags(in.Tags)),
		Metadata:   meta,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.DiningArea{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.DiningArea, error) {
	var row models.DiningAreaModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.DiningArea{}, gorm.ErrRecordNotFound
		}
		return domain.DiningArea{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in domain.DiningArea) (domain.DiningArea, error) {
	md := in.Metadata
	if md == nil {
		md = map[string]any{}
	}
	meta, _ := json.Marshal(md)
	updates := map[string]any{
		"name":        in.Name,
		"sort_order":  in.SortOrder,
		"is_favorite": in.IsFavorite,
		"tags":        pq.StringArray(utils.NormalizeTags(in.Tags)),
		"metadata":    meta,
		"updated_at":  time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&models.DiningAreaModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return domain.DiningArea{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.DiningArea{}, gorm.ErrRecordNotFound
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
	res := r.db.WithContext(ctx).Model(&models.DiningAreaModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).
		Updates(map[string]any{"deleted_at": gorm.Expr("now()"), "updated_at": gorm.Expr("now()")})
	return res.Error
}

func (r *Repository) Restore(ctx context.Context, orgID, id uuid.UUID) error {
	state, err := r.lookupState(ctx, orgID, id)
	if err != nil {
		return err
	}
	if state.DeletedAt == nil {
		return nil
	}
	res := r.db.WithContext(ctx).Model(&models.DiningAreaModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NOT NULL", orgID, id).
		Updates(map[string]any{"deleted_at": nil, "updated_at": gorm.Expr("now()")})
	return res.Error
}

func (r *Repository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Unscoped().
		Where("org_id = ? AND id = ? AND deleted_at IS NOT NULL", orgID, id).
		Delete(&models.DiningAreaModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) lookupState(ctx context.Context, orgID, id uuid.UUID) (models.DiningAreaModel, error) {
	var row models.DiningAreaModel
	err := r.db.WithContext(ctx).
		Select("id, deleted_at").
		Where("org_id = ? AND id = ?", orgID, id).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.DiningAreaModel{}, gorm.ErrRecordNotFound
		}
		return models.DiningAreaModel{}, err
	}
	return row, nil
}

func toDomain(row models.DiningAreaModel) domain.DiningArea {
	var meta map[string]any
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &meta)
	}
	if meta == nil {
		meta = map[string]any{}
	}
	return domain.DiningArea{
		ID:         row.ID,
		OrgID:   row.OrgID,
		Name:       row.Name,
		SortOrder:  row.SortOrder,
		IsFavorite: row.IsFavorite,
		Tags:       append([]string(nil), row.Tags...),
		Metadata:   meta,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
		DeletedAt:  row.DeletedAt,
	}
}
