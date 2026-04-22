package tables

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/http/go/pagination"
	utils "github.com/devpablocristo/core/validate/go/stringutil"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/restaurants/backend/internal/dining/tables/repository/models"
	domain "github.com/devpablocristo/pymes/restaurants/backend/internal/dining/tables/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, p ListParams) ([]domain.DiningTable, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&models.DiningTableModel{}).Where("org_id = ?", p.OrgID)
	if p.AreaID != nil && *p.AreaID != uuid.Nil {
		q = q.Where("area_id = ?", *p.AreaID)
	}
	if search := strings.TrimSpace(p.Search); search != "" {
		like := "%" + search + "%"
		q = q.Where("(code ILIKE ? OR label ILIKE ? OR notes ILIKE ?)", like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}
	if p.After != nil && *p.After != uuid.Nil {
		q = q.Where("id < ?", *p.After)
	}
	q = q.Order("code ASC")

	var rows []models.DiningTableModel
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]domain.DiningTable, 0, len(rows))
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

func (r *Repository) Create(ctx context.Context, in domain.DiningTable) (domain.DiningTable, error) {
	row := models.DiningTableModel{
		ID:         uuid.New(),
		OrgID:      in.OrgID,
		AreaID:     in.AreaID,
		Code:       in.Code,
		Label:      in.Label,
		Capacity:   in.Capacity,
		Status:     in.Status,
		Notes:      in.Notes,
		IsFavorite: in.IsFavorite,
		Tags:       pq.StringArray(utils.NormalizeTags(in.Tags)),
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		if httperrors.IsUniqueViolation(err) {
			return domain.DiningTable{}, ErrDuplicateTableCode
		}
		return domain.DiningTable{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.DiningTable, error) {
	var row models.DiningTableModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.DiningTable{}, gorm.ErrRecordNotFound
		}
		return domain.DiningTable{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in domain.DiningTable) (domain.DiningTable, error) {
	updates := map[string]any{
		"area_id":     in.AreaID,
		"code":        in.Code,
		"label":       in.Label,
		"capacity":    in.Capacity,
		"status":      in.Status,
		"notes":       in.Notes,
		"is_favorite": in.IsFavorite,
		"tags":        pq.StringArray(utils.NormalizeTags(in.Tags)),
		"updated_at":  time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&models.DiningTableModel{}).
		Where("org_id = ? AND id = ?", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		if httperrors.IsUniqueViolation(res.Error) {
			return domain.DiningTable{}, ErrDuplicateTableCode
		}
		return domain.DiningTable{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.DiningTable{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func (r *Repository) SetStatus(ctx context.Context, orgID, tableID uuid.UUID, status string) error {
	res := r.db.WithContext(ctx).Model(&models.DiningTableModel{}).
		Where("org_id = ? AND id = ?", orgID, tableID).
		Updates(map[string]any{"status": status, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ErrDuplicateTableCode indica código de mesa duplicado para la org (constraint único).
var ErrDuplicateTableCode = errors.New("duplicate table code")

func toDomain(row models.DiningTableModel) domain.DiningTable {
	return domain.DiningTable{
		ID:         row.ID,
		OrgID:      row.OrgID,
		AreaID:     row.AreaID,
		Code:       row.Code,
		Label:      row.Label,
		Capacity:   row.Capacity,
		Status:     row.Status,
		Notes:      row.Notes,
		IsFavorite: row.IsFavorite,
		Tags:       append([]string(nil), row.Tags...),
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
}
