package employees

import (
	"context"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	utils "github.com/devpablocristo/core/validate/go/stringutil"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/employees/repository/models"
	empdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/employees/usecases/domain"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type ListParams struct {
	OrgID  uuid.UUID
	Limit  int
	After  *uuid.UUID
	Status string
}

func (r *Repository) List(ctx context.Context, p ListParams) ([]empdomain.Employee, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&models.EmployeeModel{}).
		Where("org_id = ? AND deleted_at IS NULL", p.OrgID)
	if s := strings.TrimSpace(p.Status); s != "" {
		q = q.Where("status = ?", s)
	}
	if p.After != nil && *p.After != uuid.Nil {
		q = q.Where("id < ?", *p.After)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}
	var rows []models.EmployeeModel
	if err := q.Order("created_at DESC").Order("id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]empdomain.Employee, 0, len(rows))
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

func (r *Repository) ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]empdomain.Employee, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	var rows []models.EmployeeModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND deleted_at IS NOT NULL", orgID).
		Order("deleted_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]empdomain.Employee, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (empdomain.Employee, error) {
	var row models.EmployeeModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).
		Take(&row).Error; err != nil {
		return empdomain.Employee{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Create(ctx context.Context, in empdomain.Employee) (empdomain.Employee, error) {
	now := time.Now().UTC()
	row := models.EmployeeModel{
		ID:         uuid.New(),
		OrgID:      in.OrgID,
		FirstName:  strings.TrimSpace(in.FirstName),
		LastName:   strings.TrimSpace(in.LastName),
		Email:      strings.TrimSpace(in.Email),
		Phone:      strings.TrimSpace(in.Phone),
		Position:   strings.TrimSpace(in.Position),
		Status:     string(in.Status),
		HireDate:   in.HireDate,
		EndDate:    in.EndDate,
		UserID:     in.UserID,
		Notes:      strings.TrimSpace(in.Notes),
		IsFavorite: in.IsFavorite,
		Tags:       pq.StringArray(utils.NormalizeTags(in.Tags)),
		CreatedBy:  strings.TrimSpace(in.CreatedBy),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return empdomain.Employee{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in empdomain.Employee) (empdomain.Employee, error) {
	now := time.Now().UTC()
	updates := map[string]any{
		"first_name":  strings.TrimSpace(in.FirstName),
		"last_name":   strings.TrimSpace(in.LastName),
		"email":       strings.TrimSpace(in.Email),
		"phone":       strings.TrimSpace(in.Phone),
		"position":    strings.TrimSpace(in.Position),
		"status":      string(in.Status),
		"hire_date":   in.HireDate,
		"end_date":    in.EndDate,
		"notes":       strings.TrimSpace(in.Notes),
		"is_favorite": in.IsFavorite,
		"tags":        pq.StringArray(utils.NormalizeTags(in.Tags)),
		"updated_at":  now,
	}
	res := r.db.WithContext(ctx).Model(&models.EmployeeModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return empdomain.Employee{}, res.Error
	}
	if res.RowsAffected == 0 {
		return empdomain.Employee{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func (r *Repository) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&models.EmployeeModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).
		Update("deleted_at", now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) Restore(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&models.EmployeeModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NOT NULL", orgID, id).
		Update("deleted_at", nil)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) HardDelete(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Delete(&models.EmployeeModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func toDomain(row models.EmployeeModel) empdomain.Employee {
	return empdomain.Employee{
		ID:         row.ID,
		OrgID:      row.OrgID,
		FirstName:  row.FirstName,
		LastName:   row.LastName,
		Email:      row.Email,
		Phone:      row.Phone,
		Position:   row.Position,
		Status:     empdomain.EmployeeStatus(row.Status),
		HireDate:   row.HireDate,
		EndDate:    row.EndDate,
		UserID:     row.UserID,
		Notes:      row.Notes,
		IsFavorite: row.IsFavorite,
		Tags:       append([]string(nil), row.Tags...),
		CreatedBy:  row.CreatedBy,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
		ArchivedAt: row.DeletedAt,
	}
}
