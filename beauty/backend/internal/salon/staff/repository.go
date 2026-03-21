package staff

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/backend/go/pagination"
	"github.com/devpablocristo/pymes/beauty/backend/internal/salon/staff/repository/models"
	domain "github.com/devpablocristo/pymes/beauty/backend/internal/salon/staff/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, p ListParams) ([]domain.StaffMember, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&models.StaffMemberModel{}).Where("org_id = ?", p.OrgID)
	if search := strings.TrimSpace(p.Search); search != "" {
		like := "%" + search + "%"
		q = q.Where("(display_name ILIKE ? OR role ILIKE ? OR notes ILIKE ?)", like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}
	if p.After != nil && *p.After != uuid.Nil {
		q = q.Where("id < ?", *p.After)
	}
	q = q.Order("id DESC")

	var rows []models.StaffMemberModel
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]domain.StaffMember, 0, len(rows))
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

func (r *Repository) Create(ctx context.Context, in domain.StaffMember) (domain.StaffMember, error) {
	row := models.StaffMemberModel{
		ID:          uuid.New(),
		OrgID:       in.OrgID,
		DisplayName: in.DisplayName,
		Role:        in.Role,
		Color:       in.Color,
		IsActive:    in.IsActive,
		Notes:       in.Notes,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.StaffMember{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.StaffMember, error) {
	var row models.StaffMemberModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.StaffMember{}, gorm.ErrRecordNotFound
		}
		return domain.StaffMember{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in domain.StaffMember) (domain.StaffMember, error) {
	updates := map[string]any{
		"display_name": in.DisplayName,
		"role":         in.Role,
		"color":        in.Color,
		"is_active":    in.IsActive,
		"notes":        in.Notes,
		"updated_at":   time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&models.StaffMemberModel{}).
		Where("org_id = ? AND id = ?", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return domain.StaffMember{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.StaffMember{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func toDomain(row models.StaffMemberModel) domain.StaffMember {
	return domain.StaffMember{
		ID:          row.ID,
		OrgID:       row.OrgID,
		DisplayName: row.DisplayName,
		Role:        row.Role,
		Color:       row.Color,
		IsActive:    row.IsActive,
		Notes:       row.Notes,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
