package appointments

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/backend/go/apperror"
	appointmentsdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/appointments/usecases/domain"
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

type TimelinePort interface {
	RecordEvent(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID, eventType, title, description, actor string, metadata map[string]any) error
}

type WebhookPort interface {
	Enqueue(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) error
}

type Usecases struct {
	repo     RepositoryPort
	audit    AuditPort
	timeline TimelinePort
	webhooks WebhookPort
}

type Option func(*Usecases)

func WithTimeline(t TimelinePort) Option { return func(u *Usecases) { u.timeline = t } }
func WithWebhooks(w WebhookPort) Option  { return func(u *Usecases) { u.webhooks = w } }

func NewUsecases(repo RepositoryPort, audit AuditPort, opts ...Option) *Usecases {
	uc := &Usecases{repo: repo, audit: audit}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
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
	if u.timeline != nil && out.CustomerID != nil {
		_ = u.timeline.RecordEvent(ctx, prepared.OrgID, "parties", *out.CustomerID, "appointment.created", "Turno registrado", out.Title, prepared.CreatedBy, map[string]any{"appointment_id": out.ID.String(), "status": out.Status})
	}
	if u.webhooks != nil {
		_ = u.webhooks.Enqueue(ctx, prepared.OrgID, "appointment.created", map[string]any{"appointment_id": out.ID.String(), "customer_id": nullableUUID(out.CustomerID), "status": out.Status, "start_at": out.StartAt})
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
	if u.timeline != nil && out.CustomerID != nil {
		_ = u.timeline.RecordEvent(ctx, in.OrgID, "parties", *out.CustomerID, "appointment.updated", "Turno actualizado", out.Title, actor, map[string]any{"appointment_id": out.ID.String(), "status": out.Status})
	}
	if u.webhooks != nil {
		_ = u.webhooks.Enqueue(ctx, in.OrgID, "appointment.updated", map[string]any{"appointment_id": out.ID.String(), "customer_id": nullableUUID(out.CustomerID), "status": out.Status, "start_at": out.StartAt})
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
	if u.webhooks != nil {
		_ = u.webhooks.Enqueue(ctx, orgID, "appointment.cancelled", map[string]any{"appointment_id": id.String()})
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

func nullableUUID(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
