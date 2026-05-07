package exams

import (
	"context"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/google/uuid"
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
	q := r.db.WithContext(ctx).Model(&models.ExamModel{}).Where("tenant_id = ? AND deleted_at IS NULL", p.TenantID)
	if status := strings.TrimSpace(p.Status); status != "" {
		q = q.Where("status = ?", status)
	}
	if search := strings.TrimSpace(p.Search); search != "" {
		like := "%" + search + "%"
		q = q.Where("(patient_name ILIKE ? OR patient_document ILIKE ? OR employer_name ILIKE ? OR notes ILIKE ?)", like, like, like, like)
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
		TenantID:        in.TenantID,
		PatientName:     in.PatientName,
		PatientDocument: in.PatientDocument,
		EmployerName:    in.EmployerName,
		ExamType:        in.ExamType,
		Status:          in.Status,
		ScheduledAt:     in.ScheduledAt,
		CompletedAt:     in.CompletedAt,
		Result:          in.Result,
		Notes:           in.Notes,
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

func (r *Repository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (domain.Exam, error) {
	var row models.ExamModel
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).Take(&row).Error; err != nil {
		return domain.Exam{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in domain.Exam) (domain.Exam, error) {
	res := r.db.WithContext(ctx).Model(&models.ExamModel{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", in.TenantID, in.ID).
		Updates(map[string]any{
			"patient_name":     in.PatientName,
			"patient_document": in.PatientDocument,
			"employer_name":    in.EmployerName,
			"exam_type":        in.ExamType,
			"status":           in.Status,
			"scheduled_at":     in.ScheduledAt,
			"completed_at":     in.CompletedAt,
			"result":           in.Result,
			"notes":            in.Notes,
			"updated_by":       in.UpdatedBy,
			"updated_at":       time.Now().UTC(),
		})
	if res.Error != nil {
		return domain.Exam{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.Exam{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.TenantID, in.ID)
}

func (r *Repository) Archive(ctx context.Context, tenantID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&models.ExamModel{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
		Updates(map[string]any{"deleted_at": time.Now().UTC(), "updated_at": time.Now().UTC()})
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
		TenantID:        row.TenantID,
		PatientName:     row.PatientName,
		PatientDocument: row.PatientDocument,
		EmployerName:    row.EmployerName,
		ExamType:        row.ExamType,
		Status:          row.Status,
		ScheduledAt:     row.ScheduledAt,
		CompletedAt:     row.CompletedAt,
		Result:          row.Result,
		Notes:           row.Notes,
		CreatedBy:       row.CreatedBy,
		UpdatedBy:       row.UpdatedBy,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		DeletedAt:       deletedAt,
	}
}

var _ RepositoryPort = (*Repository)(nil)
