// Package inappnotifications implementa la bandeja in-app propia de Pymes.
package inappnotifications

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/inappnotifications/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type RepositoryPort interface {
	GetUserIDByExternalID(externalID string) (uuid.UUID, bool)
	GetOnlyUserIDByOrg(orgID uuid.UUID) (uuid.UUID, bool)
	ListForUser(orgID, userID uuid.UUID, limit int) ([]domain.InAppNotification, error)
	CountUnread(orgID, userID uuid.UUID) (int64, error)
	MarkRead(orgID, userID, notifID uuid.UUID) (time.Time, error)
}

type Usecases struct {
	repo RepositoryPort
}

func NewUsecases(repo RepositoryPort) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) resolveUserID(orgID uuid.UUID, actor string) (uuid.UUID, bool) {
	if userID, ok := u.repo.GetUserIDByExternalID(actor); ok {
		return userID, true
	}
	return u.repo.GetOnlyUserIDByOrg(orgID)
}

func (u *Usecases) ListForActor(ctx context.Context, orgIDStr, actor string, limit int) ([]domain.InAppNotification, int64, error) {
	_ = ctx
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return nil, 0, fmt.Errorf("org id: %w", httperrors.ErrBadInput)
	}
	userID, ok := u.resolveUserID(orgID, actor)
	if !ok {
		return nil, 0, fmt.Errorf("user not found: %w", httperrors.ErrNotFound)
	}
	items, err := u.repo.ListForUser(orgID, userID, limit)
	if err != nil {
		return nil, 0, err
	}
	unread, err := u.repo.CountUnread(orgID, userID)
	if err != nil {
		return nil, 0, err
	}
	return items, unread, nil
}

func (u *Usecases) MarkReadForActor(ctx context.Context, orgIDStr, actor string, notifID uuid.UUID) (time.Time, error) {
	_ = ctx
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("org id: %w", httperrors.ErrBadInput)
	}
	userID, ok := u.resolveUserID(orgID, actor)
	if !ok {
		return time.Time{}, fmt.Errorf("user not found: %w", httperrors.ErrNotFound)
	}
	readAt, err := u.repo.MarkRead(orgID, userID, notifID)
	if errors.Is(err, ErrNotFound) {
		return time.Time{}, fmt.Errorf("notification: %w", httperrors.ErrNotFound)
	}
	return readAt, err
}
