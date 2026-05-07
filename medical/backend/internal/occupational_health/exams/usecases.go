package exams

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "github.com/devpablocristo/pymes/medical/backend/internal/occupational_health/exams/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

const (
	ExamTypePreEmployment = "pre_employment"
	ExamTypePeriodic      = "periodic"
	ExamTypeReturnToWork  = "return_to_work"
	ExamTypeExit          = "exit"
	ExamTypeOther         = "other"

	StatusPending   = "pending"
	StatusScheduled = "scheduled"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
)

type ListParams struct {
	TenantID uuid.UUID
	Limit    int
	Search   string
	Status   string
}

type UpdateInput struct {
	PatientName     *string
	PatientDocument *string
	EmployerName    *string
	ExamType        *string
	Status          *string
	ScheduledAt     **time.Time
	CompletedAt     **time.Time
	Result          *string
	Notes           *string
}

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Exam, int64, error)
	Create(ctx context.Context, in domain.Exam) (domain.Exam, error)
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (domain.Exam, error)
	Update(ctx context.Context, in domain.Exam) (domain.Exam, error)
	Archive(ctx context.Context, tenantID, id uuid.UUID) error
}

type AuditPort interface {
	Log(ctx context.Context, tenantID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type Usecases struct {
	repo  RepositoryPort
	audit AuditPort
}

func NewUsecases(repo RepositoryPort, audit AuditPort) *Usecases {
	return &Usecases{repo: repo, audit: audit}
}

func (u *Usecases) List(ctx context.Context, p ListParams) ([]domain.Exam, int64, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in domain.Exam, actor string) (domain.Exam, error) {
	in.CreatedBy = actor
	in.UpdatedBy = actor
	if err := normalizeAndValidate(&in); err != nil {
		return domain.Exam{}, err
	}
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return domain.Exam{}, err
	}
	u.log(ctx, out.TenantID, actor, "medical.occupational_exam.created", out.ID, map[string]any{"status": out.Status, "type": out.ExamType})
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, tenantID, id uuid.UUID) (domain.Exam, error) {
	out, err := u.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Exam{}, fmt.Errorf("occupational exam not found: %w", httperrors.ErrNotFound)
		}
		return domain.Exam{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, tenantID, id uuid.UUID, in UpdateInput, actor string) (domain.Exam, error) {
	current, err := u.GetByID(ctx, tenantID, id)
	if err != nil {
		return domain.Exam{}, err
	}
	if in.PatientName != nil {
		current.PatientName = *in.PatientName
	}
	if in.PatientDocument != nil {
		current.PatientDocument = *in.PatientDocument
	}
	if in.EmployerName != nil {
		current.EmployerName = *in.EmployerName
	}
	if in.ExamType != nil {
		current.ExamType = *in.ExamType
	}
	if in.Status != nil {
		current.Status = *in.Status
	}
	if in.ScheduledAt != nil {
		current.ScheduledAt = *in.ScheduledAt
	}
	if in.CompletedAt != nil {
		current.CompletedAt = *in.CompletedAt
	}
	if in.Result != nil {
		current.Result = *in.Result
	}
	if in.Notes != nil {
		current.Notes = *in.Notes
	}
	current.UpdatedBy = actor
	if err := normalizeAndValidate(&current); err != nil {
		return domain.Exam{}, err
	}
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Exam{}, fmt.Errorf("occupational exam not found: %w", httperrors.ErrNotFound)
		}
		return domain.Exam{}, err
	}
	u.log(ctx, tenantID, actor, "medical.occupational_exam.updated", id, map[string]any{"status": out.Status})
	return out, nil
}

func (u *Usecases) Archive(ctx context.Context, tenantID, id uuid.UUID, actor string) error {
	if err := u.repo.Archive(ctx, tenantID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("occupational exam not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	u.log(ctx, tenantID, actor, "medical.occupational_exam.archived", id, nil)
	return nil
}

func (u *Usecases) log(ctx context.Context, tenantID uuid.UUID, actor, action string, resourceID uuid.UUID, payload map[string]any) {
	if u.audit != nil {
		u.audit.Log(ctx, tenantID.String(), actor, action, "occupational_health_exam", resourceID.String(), payload)
	}
}

func normalizeAndValidate(in *domain.Exam) error {
	in.PatientName = strings.TrimSpace(in.PatientName)
	in.PatientDocument = strings.TrimSpace(in.PatientDocument)
	in.EmployerName = strings.TrimSpace(in.EmployerName)
	in.ExamType = strings.TrimSpace(in.ExamType)
	in.Status = strings.TrimSpace(in.Status)
	in.Result = strings.TrimSpace(in.Result)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.PatientName == "" {
		return fmt.Errorf("patient_name required: %w", httperrors.ErrBadInput)
	}
	if in.ExamType == "" {
		in.ExamType = ExamTypePreEmployment
	}
	if !validExamType(in.ExamType) {
		return fmt.Errorf("invalid exam_type: %w", httperrors.ErrBadInput)
	}
	if in.Status == "" {
		in.Status = StatusPending
	}
	if !validStatus(in.Status) {
		return fmt.Errorf("invalid status: %w", httperrors.ErrBadInput)
	}
	if in.Status == StatusCompleted && in.CompletedAt == nil {
		now := time.Now().UTC()
		in.CompletedAt = &now
	}
	return nil
}

func validExamType(value string) bool {
	switch value {
	case ExamTypePreEmployment, ExamTypePeriodic, ExamTypeReturnToWork, ExamTypeExit, ExamTypeOther:
		return true
	default:
		return false
	}
}

func validStatus(value string) bool {
	switch value {
	case StatusPending, StatusScheduled, StatusCompleted, StatusCancelled:
		return true
	default:
		return false
	}
}
