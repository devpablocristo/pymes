package exams

import (
	"context"
	"strings"
	"time"

	"github.com/devpablocristo/platform/http/go/pagination"
	utils "github.com/devpablocristo/platform/validate/go/stringutil"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/medical/backend/internal/occupational_health/exams/repository/models"
	domain "github.com/devpablocristo/pymes/medical/backend/internal/occupational_health/exams/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context, p ListParams) ([]domain.Exam, int64, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	deletedClause := "deleted_at IS NULL"
	base := r.db.WithContext(ctx).Model(&models.ExamModel{})
	if p.Archived {
		deletedClause = "deleted_at IS NOT NULL"
		base = base.Unscoped()
	}
	q := base.Where("org_id = ? AND "+deletedClause, p.OrgID)
	if status := strings.TrimSpace(p.Status); status != "" {
		q = q.Where("status = ?", status)
	}
	if search := strings.TrimSpace(p.Search); search != "" {
		like := "%" + search + "%"
		q = q.Where("(patient_name ILIKE ? OR patient_document ILIKE ? OR employer_name ILIKE ? OR client_name ILIKE ? OR payment_method ILIKE ? OR notes ILIKE ? OR ? = ANY(tags))", like, like, like, like, like, like, search)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.ExamModel
	if err := q.Order("scheduled_at DESC NULLS LAST, created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]domain.Exam, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, total, nil
}

func (r *Repository) Create(ctx context.Context, in domain.Exam) (domain.Exam, error) {
	now := time.Now().UTC()
	row := models.ExamModel{
		ID:              uuid.New(),
		OrgID:           in.OrgID,
		PatientName:     in.PatientName,
		PatientDocument: in.PatientDocument,
		EmployerName:    in.EmployerName,
		ClientName:      in.ClientName,
		PaymentMethod:   in.PaymentMethod,
		ExamType:        in.ExamType,
		Status:          in.Status,
		ScheduledAt:     in.ScheduledAt,
		CompletedAt:     in.CompletedAt,
		Result:          in.Result,
		Notes:           in.Notes,
		IsFavorite:      in.IsFavorite,
		Tags:            pq.StringArray(utils.NormalizeTags(in.Tags)),
		ImageURLs:       pq.StringArray(normalizeStringSlice(in.ImageURLs)),
		CreatedBy:       in.CreatedBy,
		UpdatedBy:       in.UpdatedBy,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Exam{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Exam, error) {
	var row models.ExamModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).Take(&row).Error; err != nil {
		return domain.Exam{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in domain.Exam) (domain.Exam, error) {
	res := r.db.WithContext(ctx).Model(&models.ExamModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", in.OrgID, in.ID).
		Updates(map[string]any{
			"patient_name":     in.PatientName,
			"patient_document": in.PatientDocument,
			"employer_name":    in.EmployerName,
			"client_name":      in.ClientName,
			"payment_method":   in.PaymentMethod,
			"exam_type":        in.ExamType,
			"status":           in.Status,
			"scheduled_at":     in.ScheduledAt,
			"completed_at":     in.CompletedAt,
			"result":           in.Result,
			"notes":            in.Notes,
			"is_favorite":      in.IsFavorite,
			"tags":             pq.StringArray(utils.NormalizeTags(in.Tags)),
			"image_urls":       pq.StringArray(normalizeStringSlice(in.ImageURLs)),
			"updated_by":       in.UpdatedBy,
			"updated_at":       time.Now().UTC(),
		})
	if res.Error != nil {
		return domain.Exam{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.Exam{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func (r *Repository) Archive(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&models.ExamModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).
		Updates(map[string]any{"deleted_at": time.Now().UTC(), "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) Restore(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Unscoped().Model(&models.ExamModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NOT NULL", orgID, id).
		Updates(map[string]any{"deleted_at": nil, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) HardDelete(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Unscoped().
		Where("org_id = ? AND id = ?", orgID, id).
		Delete(&models.ExamModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func toDomain(row models.ExamModel) domain.Exam {
	var deletedAt *time.Time
	if row.DeletedAt.Valid {
		value := row.DeletedAt.Time
		deletedAt = &value
	}
	return domain.Exam{
		ID:              row.ID,
		OrgID:           row.OrgID,
		PatientName:     row.PatientName,
		PatientDocument: row.PatientDocument,
		EmployerName:    row.EmployerName,
		ClientName:      row.ClientName,
		PaymentMethod:   row.PaymentMethod,
		ExamType:        row.ExamType,
		Status:          row.Status,
		ScheduledAt:     row.ScheduledAt,
		CompletedAt:     row.CompletedAt,
		Result:          row.Result,
		Notes:           row.Notes,
		IsFavorite:      row.IsFavorite,
		Tags:            append([]string(nil), row.Tags...),
		ImageURLs:       append([]string(nil), row.ImageURLs...),
		CreatedBy:       row.CreatedBy,
		UpdatedBy:       row.UpdatedBy,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		DeletedAt:       deletedAt,
	}
}

func normalizeStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

var _ RepositoryPort = (*Repository)(nil)
