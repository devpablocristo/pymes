package inappnotifications

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	coredomain "github.com/devpablocristo/core/notifications/go/inbox/usecases/domain"
	"github.com/google/uuid"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type stubRepository struct {
	userByExternal       map[string]uuid.UUID
	onlyUserByOrg        map[uuid.UUID]uuid.UUID
	userIDsByOrg         map[uuid.UUID][]uuid.UUID
	orgIDs               []uuid.UUID
	items                []coredomain.Notification
	unread               int64
	markReadAt           time.Time
	countUnreadFn        func() int64
	listForUserCalls     []uuid.UUID
	countUnreadCalls     []uuid.UUID
	markReadUserCalls    []uuid.UUID
	markReadNotification []uuid.UUID
	resolvedOrgIDs       []string
	resolvedApprovalIDs  []string
	resolvedRequestIDs   []string
	appended             coredomain.Notification
}

type stubApprovalSource struct {
	approvals []PendingApproval
	err       error
}

func (s stubApprovalSource) ListPendingApprovals(context.Context) ([]PendingApproval, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.approvals, nil
}

func (s *stubRepository) GetUserIDByExternalID(externalID string) (uuid.UUID, bool) {
	id, ok := s.userByExternal[externalID]
	return id, ok
}

func (s *stubRepository) GetOnlyUserIDByOrg(orgID uuid.UUID) (uuid.UUID, bool) {
	id, ok := s.onlyUserByOrg[orgID]
	return id, ok
}

func (s *stubRepository) ListUserIDsByOrg(orgID uuid.UUID) ([]uuid.UUID, error) {
	list := s.userIDsByOrg[orgID]
	out := make([]uuid.UUID, len(list))
	copy(out, list)
	return out, nil
}

func (s *stubRepository) ListOrgIDsWithUsers() ([]uuid.UUID, error) {
	out := make([]uuid.UUID, len(s.orgIDs))
	copy(out, s.orgIDs)
	return out, nil
}

func (s *stubRepository) ListForRecipient(_ context.Context, tenantID, recipientID string, _ int) ([]coredomain.Notification, error) {
	orgID, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, err
	}
	userID, err := uuid.Parse(recipientID)
	if err != nil {
		return nil, err
	}
	_ = orgID
	s.listForUserCalls = append(s.listForUserCalls, userID)
	return s.items, nil
}

func (s *stubRepository) CountUnread(_ context.Context, tenantID, recipientID string) (int64, error) {
	orgID, err := uuid.Parse(tenantID)
	if err != nil {
		return 0, err
	}
	userID, err := uuid.Parse(recipientID)
	if err != nil {
		return 0, err
	}
	_ = orgID
	s.countUnreadCalls = append(s.countUnreadCalls, userID)
	if s.countUnreadFn != nil {
		return s.countUnreadFn(), nil
	}
	return s.unread, nil
}

func (s *stubRepository) Append(_ context.Context, notification coredomain.Notification) (coredomain.Notification, error) {
	s.appended = notification
	stored := notification
	if stored.ID == "" {
		stored.ID = uuid.NewString()
	} else if _, err := uuid.Parse(stored.ID); err != nil {
		stored.ID = uuid.NewSHA1(
			uuid.NameSpaceURL,
			[]byte(stored.TenantID+":"+stored.RecipientID+":"+stored.ID),
		).String()
	}
	if stored.CreatedAt.IsZero() {
		stored.CreatedAt = time.Now().UTC()
	}
	replaced := false
	for i := range s.items {
		if s.items[i].ID == stored.ID {
			s.items[i] = stored
			replaced = true
			break
		}
	}
	if !replaced {
		s.items = append([]coredomain.Notification{stored}, s.items...)
	}
	return stored, nil
}

func (s *stubRepository) MarkRead(_ context.Context, tenantID, recipientID, notificationID string, readAt time.Time) (time.Time, error) {
	orgID, err := uuid.Parse(tenantID)
	if err != nil {
		return time.Time{}, err
	}
	userID, err := uuid.Parse(recipientID)
	if err != nil {
		return time.Time{}, err
	}
	notifID, err := uuid.Parse(notificationID)
	if err != nil {
		return time.Time{}, err
	}
	_ = orgID
	s.markReadUserCalls = append(s.markReadUserCalls, userID)
	s.markReadNotification = append(s.markReadNotification, notifID)
	chosenReadAt := readAt
	if !s.markReadAt.IsZero() {
		chosenReadAt = s.markReadAt
	}
	for i := range s.items {
		if s.items[i].ID == notifID.String() {
			s.items[i].ReadAt = &chosenReadAt
			return chosenReadAt, nil
		}
	}
	if s.markReadAt.IsZero() {
		return time.Time{}, ErrNotFound
	}
	return chosenReadAt, nil
}

func (s *stubRepository) ResolveApprovalNotifications(_ context.Context, tenantID, approvalID, requestID string, readAt time.Time) (int64, error) {
	s.resolvedOrgIDs = append(s.resolvedOrgIDs, tenantID)
	s.resolvedApprovalIDs = append(s.resolvedApprovalIDs, approvalID)
	s.resolvedRequestIDs = append(s.resolvedRequestIDs, requestID)
	var affected int64
	for i := range s.items {
		if !isApprovalNotification(s.items[i]) {
			continue
		}
		if approvalID != "" && s.items[i].EntityID == approvalID {
			s.items[i].ReadAt = &readAt
			affected++
			continue
		}
		var meta struct {
			Approval struct {
				RequestID string `json:"request_id"`
			} `json:"approval"`
		}
		if json.Unmarshal(s.items[i].Metadata, &meta) == nil && meta.Approval.RequestID == requestID {
			s.items[i].ReadAt = &readAt
			affected++
		}
	}
	return affected, nil
}

func TestListForActorUsesOrgMemberFallbackForServiceActor(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	repo := &stubRepository{
		onlyUserByOrg: map[uuid.UUID]uuid.UUID{orgID: userID},
		items:         []coredomain.Notification{{ID: uuid.NewString(), RecipientID: userID.String()}},
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

func TestCreateForActorUsesResolvedRecipient(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	repo := &stubRepository{
		userByExternal: map[string]uuid.UUID{"user-ext-1": userID},
	}
	uc := NewUsecases(repo)

	created, err := uc.CreateForActor(context.Background(), orgID.String(), "user-ext-1", CreateInput{
		ID:          "insight:sales_collections:month",
		Title:       "Insight disponible",
		Body:        "Hay una novedad en ventas.",
		Kind:        "insight",
		EntityType:  "insight",
		EntityID:    "sales_collections",
		ChatContext: json.RawMessage(`{"scope":"sales_collections"}`),
	})
	if err != nil {
		t.Fatalf("CreateForActor() error = %v", err)
	}

	if created.RecipientID != userID.String() {
		t.Fatalf("expected recipient %s, got %s", userID, created.RecipientID)
	}
	if repo.appended.ID != "insight:sales_collections:month" {
		t.Fatalf("expected source id propagated, got %s", repo.appended.ID)
	}
	if string(repo.appended.Metadata) != `{"scope":"sales_collections"}` {
		t.Fatalf("expected metadata persisted, got %s", repo.appended.Metadata)
	}
}

func TestListForActorSyncsPendingApprovalsIntoInbox(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	repo := &stubRepository{
		userByExternal: map[string]uuid.UUID{"user-ext-1": userID},
	}
	repo.countUnreadFn = func() int64 {
		var unread int64
		for _, item := range repo.items {
			if item.ReadAt == nil {
				unread++
			}
		}
		return unread
	}
	uc := NewUsecases(repo, WithApprovalSource(stubApprovalSource{
		approvals: []PendingApproval{{
			ID:             "appr-1",
			OrgID:          orgID.String(),
			RequestID:      "req-1",
			ActionType:     "sales.refund",
			TargetResource: "sale-1",
			Reason:         "manual",
			RiskLevel:      "medium",
			Status:         "pending",
			CreatedAt:      "2026-04-01T12:00:00Z",
		}},
	}))

	items, unread, err := uc.ListForActor(context.Background(), orgID.String(), "user-ext-1", 50)
	if err != nil {
		t.Fatalf("ListForActor() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 synced item, got %d", len(items))
	}
	if items[0].Kind != approvalNotificationKind {
		t.Fatalf("expected approval kind, got %s", items[0].Kind)
	}
	if items[0].EntityID != "appr-1" {
		t.Fatalf("expected approval entity id, got %s", items[0].EntityID)
	}
	if unread != 1 {
		t.Fatalf("expected unread 1, got %d", unread)
	}
}

func TestSyncAllPendingApprovalsSyncsEachOrgMember(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userA := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	userB := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	repo := &stubRepository{
		orgIDs:       []uuid.UUID{orgID},
		userIDsByOrg: map[uuid.UUID][]uuid.UUID{orgID: {userA, userB}},
	}
	uc := NewUsecases(repo, WithApprovalSource(stubApprovalSource{
		approvals: []PendingApproval{{
			ID:             "appr-1",
			OrgID:          orgID.String(),
			RequestID:      "req-1",
			ActionType:     "sales.refund",
			TargetResource: "sale-1",
			Reason:         "manual",
			RiskLevel:      "medium",
			Status:         "pending",
			CreatedAt:      "2026-04-01T12:00:00Z",
		}},
	}))

	recipientCount, err := uc.SyncAllPendingApprovals(context.Background())
	if err != nil {
		t.Fatalf("SyncAllPendingApprovals() error = %v", err)
	}
	if recipientCount != 2 {
		t.Fatalf("expected 2 recipients synced, got %d", recipientCount)
	}
	if len(repo.items) != 2 {
		t.Fatalf("expected 2 approval notifications, got %d", len(repo.items))
	}
}

func TestListForActorFiltersResolvedApprovalsAfterFreshSync(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	staleNotificationID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	repo := &stubRepository{
		userByExternal: map[string]uuid.UUID{"user-ext-1": userID},
		items: []coredomain.Notification{
			{
				ID:          staleNotificationID.String(),
				TenantID:    orgID.String(),
				RecipientID: userID.String(),
				Title:       "sales.refund - sale-1",
				Body:        "manual",
				Kind:        approvalNotificationKind,
				EntityType:  approvalNotificationEntityType,
				EntityID:    "appr-stale",
				Metadata:    json.RawMessage(`{"source":"review_approval"}`),
				CreatedAt:   time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
			},
			{
				ID:          "notif-1",
				TenantID:    orgID.String(),
				RecipientID: userID.String(),
				Title:       "Insight",
				Body:        "Hay una novedad",
				Kind:        "insight",
				EntityType:  "insight",
				EntityID:    "sales_collections",
				Metadata:    json.RawMessage(`{"scope":"sales_collections"}`),
				CreatedAt:   time.Date(2026, 4, 1, 11, 0, 0, 0, time.UTC),
			},
		},
	}
	repo.countUnreadFn = func() int64 {
		var unread int64
		for _, item := range repo.items {
			if item.ReadAt == nil {
				unread++
			}
		}
		return unread
	}
	uc := NewUsecases(repo, WithApprovalSource(stubApprovalSource{approvals: nil}))

	items, unread, err := uc.ListForActor(context.Background(), orgID.String(), "user-ext-1", 50)
	if err != nil {
		t.Fatalf("ListForActor() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected only non-approval item after cleanup, got %d", len(items))
	}
	if items[0].ID != "notif-1" {
		t.Fatalf("expected notif-1, got %s", items[0].ID)
	}
	if unread != 1 {
		t.Fatalf("expected unread 1 after cleanup, got %d", unread)
	}
	if len(repo.markReadNotification) != 1 || repo.markReadNotification[0] != staleNotificationID {
		t.Fatalf("expected stale approval marked read, got %+v", repo.markReadNotification)
	}
}

func TestApplyApprovalEventCreatesPendingNotificationsForEachOrgMember(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userA := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	userB := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	repo := &stubRepository{
		userIDsByOrg: map[uuid.UUID][]uuid.UUID{orgID: {userA, userB}},
	}
	uc := NewUsecases(repo)

	affected, err := uc.ApplyApprovalEvent(context.Background(), ApprovalEvent{
		Event:          approvalEventPending,
		ApprovalID:     "appr-1",
		OrgID:          orgID.String(),
		RequestID:      "req-1",
		Decision:       "pending",
		ActionType:     "sales.refund",
		TargetResource: "sale-1",
		Reason:         "manual",
		RiskLevel:      "medium",
		CreatedAt:      "2026-04-01T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("ApplyApprovalEvent() error = %v", err)
	}
	if affected != 2 {
		t.Fatalf("expected 2 recipients affected, got %d", affected)
	}
	if len(repo.items) != 2 {
		t.Fatalf("expected 2 pending notifications, got %d", len(repo.items))
	}
}

func TestApplyApprovalEventResolvesNotificationsByRequestID(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	repo := &stubRepository{
		items: []coredomain.Notification{
			{
				ID:          uuid.NewString(),
				TenantID:    orgID.String(),
				RecipientID: userID.String(),
				Title:       "sales.refund - sale-1",
				Body:        "manual",
				Kind:        approvalNotificationKind,
				EntityType:  approvalNotificationEntityType,
				EntityID:    "appr-1",
				Metadata:    json.RawMessage(`{"source":"review_approval","approval":{"request_id":"req-1"}}`),
				CreatedAt:   time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	uc := NewUsecases(repo)

	decidedAt := "2026-04-01T13:00:00Z"
	affected, err := uc.ApplyApprovalEvent(context.Background(), ApprovalEvent{
		Event:     approvalEventResolved,
		RequestID: "req-1",
		Decision:  "approved",
		DecidedAt: &decidedAt,
	})
	if err != nil {
		t.Fatalf("ApplyApprovalEvent() error = %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected 1 notification resolved, got %d", affected)
	}
	if repo.items[0].ReadAt == nil {
		t.Fatal("expected resolved notification to be marked read")
	}
	if len(repo.resolvedRequestIDs) != 1 || repo.resolvedRequestIDs[0] != "req-1" {
		t.Fatalf("expected request_id req-1, got %+v", repo.resolvedRequestIDs)
	}
}
