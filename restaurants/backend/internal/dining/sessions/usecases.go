package sessions

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/restaurants/backend/internal/dining/sessions/usecases/domain"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]domain.TableSessionListItem, int64, error)
	OpenSession(ctx context.Context, orgID, tableID uuid.UUID, guestCount int, partyLabel, notes string) (domain.TableSession, error)
	CloseSession(ctx context.Context, orgID, sessionID uuid.UUID) (domain.TableSession, error)
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

func (u *Usecases) List(ctx context.Context, p ListParams) ([]domain.TableSessionListItem, int64, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Open(ctx context.Context, orgID, tableID uuid.UUID, guestCount int, partyLabel, notes, actor string) (domain.TableSession, error) {
	if guestCount < 1 || guestCount > 99 {
		return domain.TableSession{}, fmt.Errorf("invalid guest count: %w", httperrors.ErrBadInput)
	}
	partyLabel = strings.TrimSpace(partyLabel)
	notes = strings.TrimSpace(notes)
	out, err := u.repo.OpenSession(ctx, orgID, tableID, guestCount, partyLabel, notes)
	if err != nil {
		return domain.TableSession{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "restaurant.session.opened", "table_session", out.ID.String(), map[string]any{"table_id": tableID.String()})
	}
	return out, nil
}

func (u *Usecases) Close(ctx context.Context, orgID, sessionID uuid.UUID, actor string) (domain.TableSession, error) {
	out, err := u.repo.CloseSession(ctx, orgID, sessionID)
	if err != nil {
		return domain.TableSession{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "restaurant.session.closed", "table_session", sessionID.String(), nil)
	}
	return out, nil
}
