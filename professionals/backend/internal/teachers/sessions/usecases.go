package sessions

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	archive "github.com/devpablocristo/modules/crud/archive/go/archive"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/sessions/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Session, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.Session) (domain.Session, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Session, error)
	Update(ctx context.Context, in domain.Session) (domain.Session, error)
	BookingSessionExists(ctx context.Context, orgID, bookingID uuid.UUID) (bool, error)
	CreateNote(ctx context.Context, in domain.SessionNote) (domain.SessionNote, error)
	Archive(ctx context.Context, orgID, id uuid.UUID) error
	Restore(ctx context.Context, orgID, id uuid.UUID) error
	Delete(ctx context.Context, orgID, id uuid.UUID) error
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

func (u *Usecases) List(ctx context.Context, p ListParams) ([]domain.Session, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in domain.Session, actor string) (domain.Session, error) {
	if in.BookingID == uuid.Nil {
		return domain.Session{}, fmt.Errorf("booking_id is required: %w", httperrors.ErrBadInput)
	}
	if in.ProfileID == uuid.Nil {
		return domain.Session{}, fmt.Errorf("profile_id is required: %w", httperrors.ErrBadInput)
	}

	exists, err := u.repo.BookingSessionExists(ctx, in.OrgID, in.BookingID)
	if err != nil {
		return domain.Session{}, err
	}
	if exists {
		return domain.Session{}, fmt.Errorf("a session already exists for this booking: %w", httperrors.ErrConflict)
	}

	if in.Status == "" {
		in.Status = domain.SessionStatusScheduled
	}
	if in.Metadata == nil {
		in.Metadata = map[string]any{}
	}
	in.ServiceID = normalizeServiceID(in.ServiceID)

	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return domain.Session{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "session.created", "session", out.ID.String(), map[string]any{"booking_id": out.BookingID.String()})
	}
	return out, nil
}

func normalizeServiceID(serviceID *uuid.UUID) *uuid.UUID {
	if serviceID != nil && *serviceID != uuid.Nil {
		canonical := *serviceID
		return &canonical
	}
	return nil
}

type UpdateInput struct {
	CustomerPartyID *uuid.UUID
	ServiceID       *uuid.UUID
	Status          *string
	StartedAt       *time.Time
	EndedAt         *time.Time
	Summary         *string
	Metadata        *map[string]any
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Session, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Session{}, fmt.Errorf("session not found: %w", httperrors.ErrNotFound)
		}
		return domain.Session{}, err
	}
	if err := archive.IfArchived(out.DeletedAt, "session"); err != nil {
		return domain.Session{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.Session, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Session{}, fmt.Errorf("session not found: %w", httperrors.ErrNotFound)
		}
		return domain.Session{}, err
	}
	if err := archive.IfArchived(current.DeletedAt, "session"); err != nil {
		return domain.Session{}, err
	}
	if in.CustomerPartyID != nil {
		current.CustomerPartyID = in.CustomerPartyID
	}
	if in.ServiceID != nil {
		current.ServiceID = normalizeServiceID(in.ServiceID)
	}
	if in.Status != nil {
		status := strings.TrimSpace(*in.Status)
		if !isValidSessionStatus(status) {
			return domain.Session{}, fmt.Errorf("invalid status: %w", httperrors.ErrBadInput)
		}
		current.Status = status
	}
	if in.StartedAt != nil {
		current.StartedAt = in.StartedAt
	}
	if in.EndedAt != nil {
		current.EndedAt = in.EndedAt
	}
	if in.Summary != nil {
		current.Summary = strings.TrimSpace(*in.Summary)
	}
	if in.Metadata != nil {
		current.Metadata = *in.Metadata
	}
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Session{}, fmt.Errorf("session not found: %w", httperrors.ErrNotFound)
		}
		return domain.Session{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "session.updated", "session", out.ID.String(), map[string]any{"status": out.Status})
	}
	return out, nil
}

func (u *Usecases) Complete(ctx context.Context, orgID, id uuid.UUID, actor string) (domain.Session, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Session{}, fmt.Errorf("session not found: %w", httperrors.ErrNotFound)
		}
		return domain.Session{}, err
	}
	if err := archive.IfArchived(current.DeletedAt, "session"); err != nil {
		return domain.Session{}, err
	}
	if current.Status == domain.SessionStatusCompleted {
		return domain.Session{}, fmt.Errorf("session is already completed: %w", httperrors.ErrConflict)
	}
	if current.Status == domain.SessionStatusCancelled {
		return domain.Session{}, fmt.Errorf("cannot complete a cancelled session: %w", httperrors.ErrConflict)
	}

	current.Status = domain.SessionStatusCompleted
	if current.EndedAt == nil {
		now := time.Now().UTC()
		current.EndedAt = &now
	}

	out, err := u.repo.Update(ctx, current)
	if err != nil {
		return domain.Session{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "session.completed", "session", out.ID.String(), map[string]any{})
	}
	return out, nil
}

func (u *Usecases) CreateNote(ctx context.Context, orgID, sessionID uuid.UUID, noteType, title, body, actor string) (domain.SessionNote, error) {
	session, err := u.repo.GetByID(ctx, orgID, sessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.SessionNote{}, fmt.Errorf("session not found: %w", httperrors.ErrNotFound)
		}
		return domain.SessionNote{}, err
	}
	if err := archive.IfArchived(session.DeletedAt, "session"); err != nil {
		return domain.SessionNote{}, err
	}

	body = strings.TrimSpace(body)
	if body == "" {
		return domain.SessionNote{}, fmt.Errorf("body is required: %w", httperrors.ErrBadInput)
	}
	if noteType == "" {
		noteType = "general"
	}

	note := domain.SessionNote{
		OrgID:     orgID,
		SessionID: sessionID,
		NoteType:  noteType,
		Title:     strings.TrimSpace(title),
		Body:      body,
		CreatedBy: actor,
	}
	out, err := u.repo.CreateNote(ctx, note)
	if err != nil {
		return domain.SessionNote{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "session_note.created", "session_note", out.ID.String(), map[string]any{"session_id": sessionID.String()})
	}
	return out, nil
}

func (u *Usecases) Archive(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Archive(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("session not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "session.archived", "session", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("session not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "session.restored", "session", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) Delete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Delete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("session not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "session.deleted", "session", id.String(), map[string]any{})
	}
	return nil
}

func isValidSessionStatus(status string) bool {
	switch status {
	case domain.SessionStatusScheduled, domain.SessionStatusActive, domain.SessionStatusCompleted, domain.SessionStatusCancelled:
		return true
	default:
		return false
	}
}
