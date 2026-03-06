package specialties

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	profmodels "github.com/devpablocristo/pymes/professionals/backend/internal/professional_profiles/repository/models"
	"github.com/devpablocristo/pymes/professionals/backend/internal/specialties/repository/models"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/specialties/usecases/domain"
	"github.com/devpablocristo/pymes/pkgs/go-pkg/pagination"
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
}

func (r *Repository) List(ctx context.Context, p ListParams) ([]domain.Specialty, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, 20, 100)

	q := r.db.WithContext(ctx).Model(&models.SpecialtyModel{}).Where("org_id = ?", p.OrgID)
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
	row := models.SpecialtyModel{
		ID:          uuid.New(),
		OrgID:       in.OrgID,
		Code:        strings.TrimSpace(in.Code),
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		IsActive:    in.IsActive,
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
	updates := map[string]any{
		"code":        strings.TrimSpace(in.Code),
		"name":        strings.TrimSpace(in.Name),
		"description": strings.TrimSpace(in.Description),
		"is_active":   in.IsActive,
		"updated_at":  time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&models.SpecialtyModel{}).
		Where("org_id = ? AND id = ?", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return domain.Specialty{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.Specialty{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
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
				OrgID:       orgID,
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
	return domain.Specialty{
		ID:          row.ID,
		OrgID:       row.OrgID,
		Code:        row.Code,
		Name:        row.Name,
		Description: row.Description,
		IsActive:    row.IsActive,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
