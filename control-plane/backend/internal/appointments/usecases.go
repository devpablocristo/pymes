package appointments

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	appointmentsdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/appointments/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/pkg/apperror"
)

type RepositoryPort interface {
	List(ctx context.Context, orgID uuid.UUID, from, to *time.Time, status, assigned string, limit int) ([]appointmentsdomain.Appointment, error)
	Create(ctx context.Context, in appointmentsdomain.Appointment) (appointmentsdomain.Appointment, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (appointmentsdomain.Appointment, error)
	Update(ctx context.Context, in appointmentsdomain.Appointment) (appointmentsdomain.Appointment, error)
	Cancel(ctx context.Context, orgID, id uuid.UUID) error
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

func (u *Usecases) List(ctx context.Context, orgID uuid.UUID, from, to *time.Time, status, assigned string, limit int) ([]appointmentsdomain.Appointment, error) {
	return u.repo.List(ctx, orgID, from, to, status, assigned, limit)
}

func (u *Usecases) Create(ctx context.Context, in appointmentsdomain.Appointment) (appointmentsdomain.Appointment, error) {
	prepared, err := prepareAppointment(in, true)
	if err != nil {
		return appointmentsdomain.Appointment{}, err
	}
	if prepared.ID == uuid.Nil {
		prepared.ID = uuid.New()
	}
	out, err := u.repo.Create(ctx, prepared)
	if err != nil {
		return appointmentsdomain.Appointment{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, prepared.OrgID.String(), prepared.CreatedBy, "appointment.created", "appointment", out.ID.String(), map[string]any{"title": out.Title, "status": out.Status})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (appointmentsdomain.Appointment, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appointmentsdomain.Appointment{}, apperror.NewNotFound("appointment", id.String())
		}
		return appointmentsdomain.Appointment{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, in appointmentsdomain.Appointment, actor string) (appointmentsdomain.Appointment, error) {
	current, err := u.repo.GetByID(ctx, in.OrgID, in.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appointmentsdomain.Appointment{}, apperror.NewNotFound("appointment", in.ID.String())
		}
		return appointmentsdomain.Appointment{}, err
	}
	mergeAppointment(&current, in)
	current, err = prepareAppointment(current, false)
	if err != nil {
		return appointmentsdomain.Appointment{}, err
	}
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		return appointmentsdomain.Appointment{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), actor, "appointment.updated", "appointment", out.ID.String(), map[string]any{"status": out.Status})
	}
	return out, nil
}

func (u *Usecases) Cancel(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Cancel(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperror.NewNotFound("appointment", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "appointment.cancelled", "appointment", id.String(), nil)
	}
	return nil
}

func prepareAppointment(in appointmentsdomain.Appointment, creating bool) (appointmentsdomain.Appointment, error) {
	if creating && in.OrgID == uuid.Nil {
		return appointmentsdomain.Appointment{}, apperror.NewBadInput("org_id is required")
	}
	if strings.TrimSpace(in.CustomerName) == "" {
		return appointmentsdomain.Appointment{}, apperror.NewBadInput("customer_name is required")
	}
	if strings.TrimSpace(in.Title) == "" {
		return appointmentsdomain.Appointment{}, apperror.NewBadInput("title is required")
	}
	in.Status = normalizeStatus(in.Status)
	if in.StartAt.IsZero() {
		return appointmentsdomain.Appointment{}, apperror.NewBadInput("start_at is required")
	}
	if in.Duration <= 0 && !in.EndAt.IsZero() {
		in.Duration = int(in.EndAt.Sub(in.StartAt).Minutes())
	}
	if in.Duration <= 0 {
		in.Duration = 60
	}
	if in.Duration > 720 {
		return appointmentsdomain.Appointment{}, apperror.NewBadInput("duration must be <= 720")
	}
	endAt := in.EndAt
	if endAt.IsZero() {
		endAt = in.StartAt.Add(time.Duration(in.Duration) * time.Minute)
	}
	if !endAt.After(in.StartAt) {
		return appointmentsdomain.Appointment{}, apperror.NewBadInput("end_at must be after start_at")
	}
	if _, ok := allowedStatuses[normalizeStatus(in.Status)]; !ok {
		return appointmentsdomain.Appointment{}, apperror.NewBadInput("invalid status")
	}
	in.StartAt = in.StartAt.UTC()
	in.EndAt = endAt.UTC()
	in.Status = normalizeStatus(in.Status)
	return in, nil
}

func mergeAppointment(dst *appointmentsdomain.Appointment, patch appointmentsdomain.Appointment) {
	if patch.CustomerID != nil {
		dst.CustomerID = patch.CustomerID
	}
	if strings.TrimSpace(patch.CustomerName) != "" {
		dst.CustomerName = strings.TrimSpace(patch.CustomerName)
	}
	if patch.CustomerPhone != "" {
		dst.CustomerPhone = strings.TrimSpace(patch.CustomerPhone)
	}
	if strings.TrimSpace(patch.Title) != "" {
		dst.Title = strings.TrimSpace(patch.Title)
	}
	if patch.Description != "" {
		dst.Description = strings.TrimSpace(patch.Description)
	}
	if strings.TrimSpace(patch.Status) != "" {
		dst.Status = normalizeStatus(patch.Status)
	}
	if !patch.StartAt.IsZero() {
		dst.StartAt = patch.StartAt.UTC()
	}
	if !patch.EndAt.IsZero() {
		dst.EndAt = patch.EndAt.UTC()
	}
	if patch.Duration > 0 {
		dst.Duration = patch.Duration
	}
	if patch.Location != "" {
		dst.Location = strings.TrimSpace(patch.Location)
	}
	if patch.AssignedTo != "" {
		dst.AssignedTo = strings.TrimSpace(patch.AssignedTo)
	}
	if patch.Color != "" {
		dst.Color = strings.TrimSpace(patch.Color)
	}
	if patch.Notes != "" {
		dst.Notes = strings.TrimSpace(patch.Notes)
	}
	if patch.Metadata != nil {
		dst.Metadata = patch.Metadata
	}
	if dst.Duration <= 0 && !dst.StartAt.IsZero() && !dst.EndAt.IsZero() {
		dst.Duration = int(dst.EndAt.Sub(dst.StartAt).Minutes())
	}
	if dst.Duration <= 0 {
		dst.Duration = 60
	}
	if dst.EndAt.IsZero() && !dst.StartAt.IsZero() {
		dst.EndAt = dst.StartAt.Add(time.Duration(dst.Duration) * time.Minute)
	}
	dst.Status = normalizeStatus(dst.Status)
}

var allowedStatuses = map[string]struct{}{
	"scheduled":   {},
	"confirmed":   {},
	"in_progress": {},
	"completed":   {},
	"cancelled":   {},
	"no_show":     {},
}

func normalizeStatus(v string) string {
	status := strings.TrimSpace(strings.ToLower(v))
	if status == "" {
		return "scheduled"
	}
	return status
}

func describeParseErr(field string, err error) error {
	return fmt.Errorf("invalid %s: %w", field, err)
}
