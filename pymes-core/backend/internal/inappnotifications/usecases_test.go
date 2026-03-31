package inappnotifications

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/inappnotifications/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type stubRepository struct {
	userByExternal      map[string]uuid.UUID
	onlyUserByOrg       map[uuid.UUID]uuid.UUID
	items               []domain.InAppNotification
	unread              int64
	markReadAt          time.Time
	listForUserCalls    []uuid.UUID
	countUnreadCalls    []uuid.UUID
	markReadUserCalls   []uuid.UUID
	markReadNotification []uuid.UUID
}

func (s *stubRepository) GetUserIDByExternalID(externalID string) (uuid.UUID, bool) {
	id, ok := s.userByExternal[externalID]
	return id, ok
}

func (s *stubRepository) GetOnlyUserIDByOrg(orgID uuid.UUID) (uuid.UUID, bool) {
	id, ok := s.onlyUserByOrg[orgID]
	return id, ok
}

func (s *stubRepository) ListForUser(_ uuid.UUID, userID uuid.UUID, _ int) ([]domain.InAppNotification, error) {
	s.listForUserCalls = append(s.listForUserCalls, userID)
	return s.items, nil
}

func (s *stubRepository) CountUnread(_ uuid.UUID, userID uuid.UUID) (int64, error) {
	s.countUnreadCalls = append(s.countUnreadCalls, userID)
	return s.unread, nil
}

func (s *stubRepository) MarkRead(_ uuid.UUID, userID, notifID uuid.UUID) (time.Time, error) {
	s.markReadUserCalls = append(s.markReadUserCalls, userID)
	s.markReadNotification = append(s.markReadNotification, notifID)
	if s.markReadAt.IsZero() {
		return time.Time{}, ErrNotFound
	}
	return s.markReadAt, nil
}

func TestListForActorUsesOrgMemberFallbackForServiceActor(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	repo := &stubRepository{
		onlyUserByOrg: map[uuid.UUID]uuid.UUID{orgID: userID},
		items:         []domain.InAppNotification{{ID: uuid.New(), UserID: userID}},
		unread:        1,
	}
	uc := NewUsecases(repo)

	items, unread, err := uc.ListForActor(context.Background(), orgID.String(), "api_key:"+orgID.String(), 50)
	if err != nil {
		t.Fatalf("ListForActor() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if unread != 1 {
		t.Fatalf("expected unread 1, got %d", unread)
	}
	if len(repo.listForUserCalls) != 1 || repo.listForUserCalls[0] != userID {
		t.Fatalf("expected fallback user %s in list call, got %+v", userID, repo.listForUserCalls)
	}
	if len(repo.countUnreadCalls) != 1 || repo.countUnreadCalls[0] != userID {
		t.Fatalf("expected fallback user %s in unread call, got %+v", userID, repo.countUnreadCalls)
	}
}

func TestMarkReadForActorUsesOrgMemberFallbackForServiceActor(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	notifID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	expectedReadAt := time.Date(2026, 3, 31, 11, 0, 0, 0, time.UTC)
	repo := &stubRepository{
		onlyUserByOrg: map[uuid.UUID]uuid.UUID{orgID: userID},
		markReadAt:    expectedReadAt,
	}
	uc := NewUsecases(repo)

	readAt, err := uc.MarkReadForActor(context.Background(), orgID.String(), "api_key:"+orgID.String(), notifID)
	if err != nil {
		t.Fatalf("MarkReadForActor() error = %v", err)
	}
	if !readAt.Equal(expectedReadAt) {
		t.Fatalf("expected readAt %s, got %s", expectedReadAt, readAt)
	}
	if len(repo.markReadUserCalls) != 1 || repo.markReadUserCalls[0] != userID {
		t.Fatalf("expected fallback user %s in mark read call, got %+v", userID, repo.markReadUserCalls)
	}
	if len(repo.markReadNotification) != 1 || repo.markReadNotification[0] != notifID {
		t.Fatalf("expected notifID %s, got %+v", notifID, repo.markReadNotification)
	}
}

func TestListForActorReturnsNotFoundWhenNoOrgMemberFallbackExists(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	uc := NewUsecases(&stubRepository{
		onlyUserByOrg: map[uuid.UUID]uuid.UUID{},
	})

	_, _, err := uc.ListForActor(context.Background(), orgID.String(), "api_key:"+orgID.String(), 50)
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}
