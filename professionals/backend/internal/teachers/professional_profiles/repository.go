package professional_profiles

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/utils/go/pagination"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/professional_profiles/repository/models"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/professional_profiles/usecases/domain"
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

func (r *Repository) List(ctx context.Context, p ListParams) ([]domain.ProfessionalProfile, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})

	q := r.db.WithContext(ctx).Model(&models.ProfessionalProfileModel{}).Where("org_id = ?", p.OrgID)
	if s := strings.TrimSpace(p.Search); s != "" {
		like := "%" + s + "%"
		q = q.Where("(headline ILIKE ? OR bio ILIKE ? OR public_slug ILIKE ?)", like, like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}

	if p.After != nil && *p.After != uuid.Nil {
		q = q.Where("id < ?", *p.After)
	}
	q = q.Order("id DESC")

	var rows []models.ProfessionalProfileModel
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	out := make([]domain.ProfessionalProfile, 0, len(rows))
	for _, row := range rows {
		profile := toDomain(row)
		profile.Specialties = r.loadSpecialties(ctx, row.OrgID, row.ID)
		out = append(out, profile)
	}

	var next *uuid.UUID
	if hasMore && len(rows) > 0 {
		v := rows[len(rows)-1].ID
		next = &v
	}
	return out, total, hasMore, next, nil
}

func (r *Repository) Create(ctx context.Context, in domain.ProfessionalProfile) (domain.ProfessionalProfile, error) {
	meta, _ := json.Marshal(in.Metadata)
	row := models.ProfessionalProfileModel{
		ID:                uuid.New(),
		OrgID:             in.OrgID,
		PartyID:           in.PartyID,
		PublicSlug:        strings.TrimSpace(in.PublicSlug),
		Bio:               strings.TrimSpace(in.Bio),
		Headline:          strings.TrimSpace(in.Headline),
		IsPublic:          in.IsPublic,
		IsBookable:        in.IsBookable,
		AcceptsNewClients: in.AcceptsNewClients,
		Metadata:          meta,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.ProfessionalProfile{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.ProfessionalProfile, error) {
	var row models.ProfessionalProfileModel
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ProfessionalProfile{}, gorm.ErrRecordNotFound
		}
		return domain.ProfessionalProfile{}, err
	}
	profile := toDomain(row)
	profile.Specialties = r.loadSpecialties(ctx, row.OrgID, row.ID)
	return profile, nil
}

func (r *Repository) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (domain.ProfessionalProfile, error) {
	var row models.ProfessionalProfileModel
	err := r.db.WithContext(ctx).Where("org_id = ? AND public_slug = ?", orgID, slug).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ProfessionalProfile{}, gorm.ErrRecordNotFound
		}
		return domain.ProfessionalProfile{}, err
	}
	profile := toDomain(row)
	profile.Specialties = r.loadSpecialties(ctx, row.OrgID, row.ID)
	return profile, nil
}

func (r *Repository) SlugExists(ctx context.Context, orgID uuid.UUID, slug string, excludeID *uuid.UUID) (bool, error) {
	q := r.db.WithContext(ctx).Model(&models.ProfessionalProfileModel{}).Where("org_id = ? AND public_slug = ?", orgID, slug)
	if excludeID != nil {
		q = q.Where("id != ?", *excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) Update(ctx context.Context, in domain.ProfessionalProfile) (domain.ProfessionalProfile, error) {
	meta, _ := json.Marshal(in.Metadata)
	updates := map[string]any{
		"public_slug":         strings.TrimSpace(in.PublicSlug),
		"bio":                 strings.TrimSpace(in.Bio),
		"headline":            strings.TrimSpace(in.Headline),
		"is_public":           in.IsPublic,
		"is_bookable":         in.IsBookable,
		"accepts_new_clients": in.AcceptsNewClients,
		"metadata":            meta,
		"updated_at":          time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&models.ProfessionalProfileModel{}).
		Where("org_id = ? AND id = ?", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return domain.ProfessionalProfile{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ProfessionalProfile{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func (r *Repository) ListPublic(ctx context.Context, orgID uuid.UUID) ([]domain.ProfessionalProfile, error) {
	var rows []models.ProfessionalProfileModel
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND is_public = true", orgID).
		Order("headline ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.ProfessionalProfile, 0, len(rows))
	for _, row := range rows {
		profile := toDomain(row)
		profile.Specialties = r.loadSpecialties(ctx, row.OrgID, row.ID)
		out = append(out, profile)
	}
	return out, nil
}

func (r *Repository) loadSpecialties(ctx context.Context, orgID, profileID uuid.UUID) []domain.Specialty {
	var joins []models.ProfessionalSpecialtyModel
	r.db.WithContext(ctx).Where("org_id = ? AND profile_id = ?", orgID, profileID).Find(&joins)

	if len(joins) == 0 {
		return nil
	}

	specIDs := make([]uuid.UUID, 0, len(joins))
	for _, j := range joins {
		specIDs = append(specIDs, j.SpecialtyID)
	}

	type specRow struct {
		ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
		OrgID       uuid.UUID `gorm:"type:uuid"`
		Code        string
		Name        string
		Description string
		IsActive    bool
		CreatedAt   time.Time
		UpdatedAt   time.Time
	}

	var specs []specRow
	r.db.WithContext(ctx).Table("professionals.specialties").Where("id IN ?", specIDs).Find(&specs)

	out := make([]domain.Specialty, 0, len(specs))
	for _, s := range specs {
		out = append(out, domain.Specialty{
			ID:          s.ID,
			OrgID:       s.OrgID,
			Code:        s.Code,
			Name:        s.Name,
			Description: s.Description,
			IsActive:    s.IsActive,
			CreatedAt:   s.CreatedAt,
			UpdatedAt:   s.UpdatedAt,
		})
	}
	return out
}

func toDomain(row models.ProfessionalProfileModel) domain.ProfessionalProfile {
	meta := map[string]any{}
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &meta)
	}
	if meta == nil {
		meta = map[string]any{}
	}
	return domain.ProfessionalProfile{
		ID:                row.ID,
		OrgID:             row.OrgID,
		PartyID:           row.PartyID,
		PublicSlug:        row.PublicSlug,
		Bio:               row.Bio,
		Headline:          row.Headline,
		IsPublic:          row.IsPublic,
		IsBookable:        row.IsBookable,
		AcceptsNewClients: row.AcceptsNewClients,
		Metadata:          meta,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}
