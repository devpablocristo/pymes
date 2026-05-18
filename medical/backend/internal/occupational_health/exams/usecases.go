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
	OrgID uuid.UUID
	Limit    int
	Search   string
	Status   string
	Archived bool
}

type UpdateInput struct {
	PatientName     *string
	PatientDocument *string
	EmployerName    *string
	ClientName      *string
	PaymentMethod   *string
	ExamType        *string
	Status          *string
	ScheduledAt     **time.Time
	CompletedAt     **time.Time
	Result          *string
	Notes           *string
	IsFavorite      *bool
	Tags            *[]string
	ImageURLs       *[]string
}

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Exam, int64, error)
	Create(ctx context.Context, in domain.Exam) (domain.Exam, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Exam, error)
	Update(ctx context.Context, in domain.Exam) (domain.Exam, error)
	Archive(ctx context.Context, orgID, id uuid.UUID) error
	Restore(ctx context.Context, orgID, id uuid.UUID) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID) error
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
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
	u.log(ctx, out.OrgID, actor, "medical.occupational_exam.created", out.ID, map[string]any{"status": out.Status, "type": out.ExamType})
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Exam, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Exam{}, fmt.Errorf("occupational exam not found: %w", httperrors.ErrNotFound)
		}
		return domain.Exam{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.Exam, error) {
	current, err := u.GetByID(ctx, orgID, id)
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
	if in.ClientName != nil {
		current.ClientName = *in.ClientName
	}
	if in.PaymentMethod != nil {
		current.PaymentMethod = *in.PaymentMethod
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
	if in.IsFavorite != nil {
		current.IsFavorite = *in.IsFavorite
	}
	if in.Tags != nil {
		current.Tags = append([]string(nil), (*in.Tags)...)
	}
	if in.ImageURLs != nil {
		current.ImageURLs = append([]string(nil), (*in.ImageURLs)...)
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
	u.log(ctx, orgID, actor, "medical.occupational_exam.updated", id, map[string]any{"status": out.Status})
	return out, nil
}

func (u *Usecases) Archive(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Archive(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("occupational exam not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	u.log(ctx, orgID, actor, "medical.occupational_exam.archived", id, nil)
	return nil
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("occupational exam not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	u.log(ctx, orgID, actor, "medical.occupational_exam.restored", id, nil)
	return nil
}

func (u *Usecases) HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.HardDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("occupational exam not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	u.log(ctx, orgID, actor, "medical.occupational_exam.hard_deleted", id, nil)
	return nil
}

func (u *Usecases) log(ctx context.Context, orgID uuid.UUID, actor, action string, resourceID uuid.UUID, payload map[string]any) {
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, action, "occupational_health_exam", resourceID.String(), payload)
	}
}

func normalizeAndValidate(in *domain.Exam) error {
	in.PatientName = strings.TrimSpace(in.PatientName)
	in.PatientDocument = strings.TrimSpace(in.PatientDocument)
	in.EmployerName = strings.TrimSpace(in.EmployerName)
	in.ClientName = strings.TrimSpace(in.ClientName)
	in.PaymentMethod = strings.TrimSpace(in.PaymentMethod)
	in.ExamType = strings.TrimSpace(in.ExamType)
	in.Status = strings.TrimSpace(in.Status)
	in.Result = strings.TrimSpace(in.Result)
	in.Notes = strings.TrimSpace(in.Notes)
	in.Tags = normalizeStringList(in.Tags)
	in.ImageURLs = normalizeStringList(in.ImageURLs)
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

func normalizeStringList(values []string) []string {
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
