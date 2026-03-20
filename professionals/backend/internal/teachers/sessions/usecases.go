package sessions

import (
	"errors"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/sessions/usecases/domain"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Session, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.Session) (domain.Session, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Session, error)
	Update(ctx context.Context, in domain.Session) (domain.Session, error)
	AppointmentSessionExists(ctx context.Context, orgID, appointmentID uuid.UUID) (bool, error)
	CreateNote(ctx context.Context, in domain.SessionNote) (domain.SessionNote, error)
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
	if in.AppointmentID == uuid.Nil {
		return domain.Session{}, fmt.Errorf("appointment_id is required: %w", httperrors.ErrBadInput)
	}
	if in.ProfileID == uuid.Nil {
		return domain.Session{}, fmt.Errorf("profile_id is required: %w", httperrors.ErrBadInput)
	}

	exists, err := u.repo.AppointmentSessionExists(ctx, in.OrgID, in.AppointmentID)
	if err != nil {
		return domain.Session{}, err
	}
	if exists {
		return domain.Session{}, fmt.Errorf("a session already exists for this appointment: %w", httperrors.ErrConflict)
	}

	if in.Status == "" {
		in.Status = domain.SessionStatusScheduled
	}
	if in.Metadata == nil {
		in.Metadata = map[string]any{}
	}

	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return domain.Session{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "session.created", "session", out.ID.String(), map[string]any{"appointment_id": out.AppointmentID.String()})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Session, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Session{}, fmt.Errorf("session not found: %w", httperrors.ErrNotFound)
		}
		return domain.Session{}, err
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
	if _, err := u.repo.GetByID(ctx, orgID, sessionID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.SessionNote{}, fmt.Errorf("session not found: %w", httperrors.ErrNotFound)
		}
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
