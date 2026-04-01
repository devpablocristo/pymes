// Package inappnotifications implementa la bandeja in-app propia de Pymes.
package inappnotifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	coreinbox "github.com/devpablocristo/core/notifications/go/inbox"
	coredomain "github.com/devpablocristo/core/notifications/go/inbox/usecases/domain"
	"github.com/google/uuid"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type RepositoryPort interface {
	coreinbox.Repository
	GetUserIDByExternalID(externalID string) (uuid.UUID, bool)
	GetOnlyUserIDByOrg(orgID uuid.UUID) (uuid.UUID, bool)
	ListUserIDsByOrg(orgID uuid.UUID) ([]uuid.UUID, error)
	ListOrgIDsWithUsers() ([]uuid.UUID, error)
	ResolveApprovalNotifications(ctx context.Context, tenantID, approvalID, requestID string, readAt time.Time) (int64, error)
}

type CreateInput struct {
	ID          string
	Title       string
	Body        string
	Kind        string
	EntityType  string
	EntityID    string
	ChatContext json.RawMessage
}

type Usecases struct {
	repo           RepositoryPort
	inbox          *coreinbox.Usecases
	approvalSource ApprovalSource
}

type Option func(*Usecases)

func WithApprovalSource(source ApprovalSource) Option {
	return func(uc *Usecases) {
		uc.approvalSource = source
	}
}

func NewUsecases(repo RepositoryPort, opts ...Option) *Usecases {
	uc := &Usecases{
		repo:  repo,
		inbox: coreinbox.NewUsecases(repo),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(uc)
		}
	}
	return uc
}

func (u *Usecases) resolveUserID(orgID uuid.UUID, actor string) (uuid.UUID, bool) {
	if userID, ok := u.repo.GetUserIDByExternalID(actor); ok {
		return userID, true
	}
	return u.repo.GetOnlyUserIDByOrg(orgID)
}

func (u *Usecases) ListForActor(ctx context.Context, orgIDStr, actor string, limit int) ([]coredomain.Notification, int64, error) {
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return nil, 0, fmt.Errorf("org id: %w", httperrors.ErrBadInput)
	}
	userID, ok := u.resolveUserID(orgID, actor)
	if !ok {
		return nil, 0, fmt.Errorf("user not found: %w", httperrors.ErrNotFound)
	}
	pendingApprovals, approvalsFresh := u.syncPendingApprovals(ctx, orgID.String(), userID.String())
	items, err := u.inbox.ListForRecipient(ctx, orgID.String(), userID.String(), approvalSyncLimit)
	if err != nil {
		return nil, 0, err
	}
	items = u.filterResolvedApprovals(ctx, orgID.String(), userID.String(), items, pendingApprovals, approvalsFresh)
	unread, err := u.inbox.CountUnread(ctx, orgID.String(), userID.String())
	if err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > approvalSyncLimit {
		limit = 100
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, unread, nil
}

func (u *Usecases) MarkReadForActor(ctx context.Context, orgIDStr, actor string, notifID uuid.UUID) (time.Time, error) {
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("org id: %w", httperrors.ErrBadInput)
	}
	userID, ok := u.resolveUserID(orgID, actor)
	if !ok {
		return time.Time{}, fmt.Errorf("user not found: %w", httperrors.ErrNotFound)
	}
	readAt, err := u.inbox.MarkRead(ctx, orgID.String(), userID.String(), notifID.String())
	if errors.Is(err, ErrNotFound) {
		return time.Time{}, fmt.Errorf("notification: %w", httperrors.ErrNotFound)
	}
	return readAt, err
}

func (u *Usecases) CreateForActor(ctx context.Context, orgIDStr, actor string, input CreateInput) (coredomain.Notification, error) {
	orgID, err := uuid.Parse(strings.TrimSpace(orgIDStr))
	if err != nil {
		return coredomain.Notification{}, fmt.Errorf("org id: %w", httperrors.ErrBadInput)
	}
	userID, ok := u.resolveUserID(orgID, actor)
	if !ok {
		return coredomain.Notification{}, fmt.Errorf("user not found: %w", httperrors.ErrNotFound)
	}
	if len(input.ChatContext) > 0 && !json.Valid(input.ChatContext) {
		return coredomain.Notification{}, fmt.Errorf("chat context: %w", httperrors.ErrBadInput)
	}
	return u.inbox.Create(ctx, coredomain.Notification{
		ID:          input.ID,
		TenantID:    orgID.String(),
		RecipientID: userID.String(),
		Title:       input.Title,
		Body:        input.Body,
		Kind:        input.Kind,
		EntityType:  input.EntityType,
		EntityID:    input.EntityID,
		Metadata:    input.ChatContext,
	})
}

func (u *Usecases) ApplyApprovalEvent(ctx context.Context, event ApprovalEvent) (int, error) {
	switch normalizeApprovalEvent(event) {
	case approvalEventPending:
		orgID, err := uuid.Parse(strings.TrimSpace(event.OrgID))
		if err != nil {
			return 0, fmt.Errorf("org id: %w", httperrors.ErrBadInput)
		}
		recipients, err := u.repo.ListUserIDsByOrg(orgID)
		if err != nil {
			return 0, err
		}
		approval := approvalEventToPendingApproval(event)
		affected := 0
		for _, recipientID := range recipients {
			u.applyApprovalSnapshot(ctx, orgID.String(), recipientID.String(), []PendingApproval{approval})
			affected++
		}
		return affected, nil
	case approvalEventResolved:
		readAt := time.Now().UTC()
		if event.DecidedAt != nil {
			if parsed := parseApprovalCreatedAt(*event.DecidedAt); !parsed.IsZero() {
				readAt = parsed
			}
		}
		affected, err := u.repo.ResolveApprovalNotifications(
			ctx,
			strings.TrimSpace(event.OrgID),
			strings.TrimSpace(event.ApprovalID),
			strings.TrimSpace(event.RequestID),
			readAt,
		)
		return int(affected), err
	default:
		return 0, fmt.Errorf("approval event: %w", httperrors.ErrBadInput)
	}
}

func (u *Usecases) syncPendingApprovals(
	ctx context.Context,
	tenantID string,
	recipientID string,
) (map[string]struct{}, bool) {
	if u.approvalSource == nil {
		return nil, false
	}
	approvals, err := u.approvalSource.ListPendingApprovals(ctx)
	if err != nil {
		return nil, false
	}
	selected := filterApprovalsForTenant(tenantID, approvals, true)
	return u.applyApprovalSnapshot(ctx, tenantID, recipientID, selected), true
}

func (u *Usecases) SyncAllPendingApprovals(ctx context.Context) (int, error) {
	if u.approvalSource == nil {
		return 0, nil
	}
	approvals, err := u.approvalSource.ListPendingApprovals(ctx)
	if err != nil {
		return 0, err
	}
	grouped := groupApprovalsByTenant(approvals)
	orgIDs, err := u.repo.ListOrgIDsWithUsers()
	if err != nil {
		return 0, err
	}
	totalRecipients := 0
	for _, orgID := range orgIDs {
		tenantID := orgID.String()
		selected := grouped[tenantID]
		pending := make(map[string]struct{}, len(selected))
		for _, approval := range selected {
			if approvalID := strings.TrimSpace(approval.ID); approvalID != "" {
				pending[approvalID] = struct{}{}
			}
		}
		recipientIDs, err := u.repo.ListUserIDsByOrg(orgID)
		if err != nil {
			return totalRecipients, err
		}
		for _, recipientID := range recipientIDs {
			recipientIDStr := recipientID.String()
			u.applyApprovalSnapshot(ctx, tenantID, recipientIDStr, selected)
			items, err := u.inbox.ListForRecipient(ctx, tenantID, recipientIDStr, approvalSyncLimit)
			if err != nil {
				return totalRecipients, err
			}
			_ = u.filterResolvedApprovals(ctx, tenantID, recipientIDStr, items, pending, true)
			totalRecipients++
		}
	}
	return totalRecipients, nil
}

func (u *Usecases) applyApprovalSnapshot(
	ctx context.Context,
	tenantID string,
	recipientID string,
	approvals []PendingApproval,
) map[string]struct{} {
	pending := make(map[string]struct{}, len(approvals))
	for _, approval := range approvals {
		approvalID := strings.TrimSpace(approval.ID)
		if approvalID == "" {
			continue
		}
		pending[approvalID] = struct{}{}
		_, _ = u.inbox.Create(ctx, buildApprovalNotification(tenantID, recipientID, approval))
	}
	return pending
}

func (u *Usecases) filterResolvedApprovals(
	ctx context.Context,
	tenantID string,
	recipientID string,
	items []coredomain.Notification,
	pending map[string]struct{},
	approvalsFresh bool,
) []coredomain.Notification {
	if !approvalsFresh {
		return items
	}
	filtered := make([]coredomain.Notification, 0, len(items))
	for _, item := range items {
		if !isApprovalNotification(item) {
			filtered = append(filtered, item)
			continue
		}
		if _, ok := pending[strings.TrimSpace(item.EntityID)]; ok {
			filtered = append(filtered, item)
			continue
		}
		if item.ReadAt == nil {
			_, _ = u.inbox.MarkRead(ctx, tenantID, recipientID, item.ID)
		}
	}
	return filtered
}

func filterApprovalsForTenant(tenantID string, approvals []PendingApproval, includeOrgless bool) []PendingApproval {
	out := make([]PendingApproval, 0, len(approvals))
	tenantID = strings.TrimSpace(tenantID)
	for _, approval := range approvals {
		orgID := strings.TrimSpace(approval.OrgID)
		switch {
		case orgID == "" && includeOrgless:
			out = append(out, approval)
		case orgID == tenantID:
			out = append(out, approval)
		}
	}
	return out
}

func groupApprovalsByTenant(approvals []PendingApproval) map[string][]PendingApproval {
	grouped := make(map[string][]PendingApproval)
	for _, approval := range approvals {
		orgID := strings.TrimSpace(approval.OrgID)
		if orgID == "" {
			continue
		}
		grouped[orgID] = append(grouped[orgID], approval)
	}
	return grouped
}
