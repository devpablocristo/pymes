package specialties

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
	profmodels "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/professional_profiles/repository/models"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/specialties/repository/models"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/specialties/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type ListParams struct {
	OrgID uuid.UUID
	Limit    int
	After    *uuid.UUID
	Search   string
	Archived bool
}

func (r *Repository) List(ctx context.Context, p ListParams) ([]domain.Specialty, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})

	q := r.db.WithContext(ctx).Model(&models.SpecialtyModel{}).Where("org_id = ?", p.OrgID)
	if p.Archived {
		q = q.Where("deleted_at IS NOT NULL")
	} else {
		q = q.Where("deleted_at IS NULL")
	}
	if s := strings.TrimSpace(p.Search); s != "" {
		like := "%" + s + "%"
		q = q.Where("(name ILIKE ? OR code ILIKE ? OR description ILIKE ?)", like, like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}

	if p.After != nil && *p.After != uuid.Nil {
		q = q.Where("id < ?", *p.After)
	}
	q = q.Order("id DESC")

	var rows []models.SpecialtyModel
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	out := make([]domain.Specialty, 0, len(rows))
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

func (r *Repository) Create(ctx context.Context, in domain.Specialty) (domain.Specialty, error) {
	md := in.Metadata
	if md == nil {
		md = map[string]any{}
	}
	meta, _ := json.Marshal(md)
	row := models.SpecialtyModel{
		ID:          uuid.New(),
		OrgID:    in.OrgID,
		Code:        strings.TrimSpace(in.Code),
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		IsActive:    in.IsActive,
		IsFavorite:  in.IsFavorite,
		Tags:        pq.StringArray(utils.NormalizeTags(in.Tags)),
		Metadata:    meta,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Specialty{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Specialty, error) {
	var row models.SpecialtyModel
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Specialty{}, gorm.ErrRecordNotFound
		}
		return domain.Specialty{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in domain.Specialty) (domain.Specialty, error) {
	md := in.Metadata
	if md == nil {
		md = map[string]any{}
	}
	meta, _ := json.Marshal(md)
	updates := map[string]any{
		"code":        strings.TrimSpace(in.Code),
		"name":        strings.TrimSpace(in.Name),
		"description": strings.TrimSpace(in.Description),
		"is_active":   in.IsActive,
		"is_favorite": in.IsFavorite,
		"tags":        pq.StringArray(utils.NormalizeTags(in.Tags)),
		"metadata":    meta,
		"updated_at":  time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&models.SpecialtyModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return domain.Specialty{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.Specialty{}, gorm.ErrRecordNotFound
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
	res := r.db.WithContext(ctx).Model(&models.SpecialtyModel{}).
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
	res := r.db.WithContext(ctx).Model(&models.SpecialtyModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NOT NULL", orgID, id).
		Updates(map[string]any{"deleted_at": nil, "updated_at": gorm.Expr("now()")})
	return res.Error
}

func (r *Repository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Unscoped().
		Where("org_id = ? AND id = ? AND deleted_at IS NOT NULL", orgID, id).
		Delete(&models.SpecialtyModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) lookupState(ctx context.Context, orgID, id uuid.UUID) (models.SpecialtyModel, error) {
	var row models.SpecialtyModel
	err := r.db.WithContext(ctx).
		Select("id, deleted_at").
		Where("org_id = ? AND id = ?", orgID, id).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.SpecialtyModel{}, gorm.ErrRecordNotFound
		}
		return models.SpecialtyModel{}, err
	}
	return row, nil
}

func (r *Repository) CodeExists(ctx context.Context, orgID uuid.UUID, code string, excludeID *uuid.UUID) (bool, error) {
	q := r.db.WithContext(ctx).Model(&models.SpecialtyModel{}).Where("org_id = ? AND code = ?", orgID, code)
	if excludeID != nil {
		q = q.Where("id != ?", *excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) AssignProfessionals(ctx context.Context, orgID, specialtyID uuid.UUID, profileIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("org_id = ? AND specialty_id = ?", orgID, specialtyID).
			Delete(&profmodels.ProfessionalSpecialtyModel{}).Error; err != nil {
			return err
		}
		for _, pid := range profileIDs {
			join := profmodels.ProfessionalSpecialtyModel{
				ID:          uuid.New(),
				OrgID:    orgID,
				ProfileID:   pid,
				SpecialtyID: specialtyID,
				CreatedAt:   time.Now().UTC(),
			}
			if err := tx.Create(&join).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func toDomain(row models.SpecialtyModel) domain.Specialty {
	var meta map[string]any
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &meta)
	}
	if meta == nil {
		meta = map[string]any{}
	}
	return domain.Specialty{
		ID:          row.ID,
		OrgID:    row.OrgID,
		Code:        row.Code,
		Name:        row.Name,
		Description: row.Description,
		IsActive:    row.IsActive,
		IsFavorite:  row.IsFavorite,
		Tags:        append([]string(nil), row.Tags...),
		Metadata:    meta,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		DeletedAt:   row.DeletedAt,
	}
}
