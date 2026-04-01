package inappnotifications

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	coredomain "github.com/devpablocristo/core/notifications/go/inbox/usecases/domain"
)

const (
	approvalNotificationKind       = "approval"
	approvalNotificationEntityType = "review_approval"
	approvalNotificationSource     = "review_approval"
	approvalSyncLimit              = 200
	approvalEventPending           = "approval_pending"
	approvalEventResolved          = "approval_resolved"
)

type PendingApproval struct {
	ID             string
	OrgID          string
	RequestID      string
	ActionType     string
	TargetResource string
	Reason         string
	RiskLevel      string
	Status         string
	AISummary      *string
	CreatedAt      string
	ExpiresAt      *string
}

type ApprovalSource interface {
	ListPendingApprovals(ctx context.Context) ([]PendingApproval, error)
}

type ApprovalEvent struct {
	Event          string  `json:"event"`
	ApprovalID     string  `json:"approval_id,omitempty"`
	OrgID          string  `json:"org_id,omitempty"`
	RequestID      string  `json:"request_id"`
	Decision       string  `json:"decision,omitempty"`
	DecidedBy      string  `json:"decided_by,omitempty"`
	DecisionNote   string  `json:"decision_note,omitempty"`
	ActionType     string  `json:"action_type,omitempty"`
	TargetResource string  `json:"target_resource,omitempty"`
	Reason         string  `json:"reason,omitempty"`
	RiskLevel      string  `json:"risk_level,omitempty"`
	AISummary      *string `json:"ai_summary,omitempty"`
	CreatedAt      string  `json:"created_at,omitempty"`
	ExpiresAt      *string `json:"expires_at,omitempty"`
	DecidedAt      *string `json:"decided_at,omitempty"`
}

type approvalMetadata struct {
	Source   string                  `json:"source"`
	Approval approvalMetadataPayload `json:"approval"`
}

type approvalMetadataPayload struct {
	ID             string  `json:"id"`
	OrgID          string  `json:"org_id,omitempty"`
	RequestID      string  `json:"request_id"`
	ActionType     string  `json:"action_type"`
	TargetResource string  `json:"target_resource"`
	Reason         string  `json:"reason"`
	RiskLevel      string  `json:"risk_level"`
	Status         string  `json:"status"`
	AISummary      *string `json:"ai_summary,omitempty"`
	CreatedAt      string  `json:"created_at"`
	ExpiresAt      *string `json:"expires_at,omitempty"`
}

func approvalNotificationID(approvalID string) string {
	return approvalNotificationSource + ":" + strings.TrimSpace(approvalID)
}

func buildApprovalNotification(tenantID, recipientID string, approval PendingApproval) coredomain.Notification {
	return coredomain.Notification{
		ID:          approvalNotificationID(approval.ID),
		TenantID:    tenantID,
		RecipientID: recipientID,
		Title:       buildApprovalTitle(approval),
		Body:        buildApprovalBody(approval),
		Kind:        approvalNotificationKind,
		EntityType:  approvalNotificationEntityType,
		EntityID:    strings.TrimSpace(approval.ID),
		Metadata:    buildApprovalMetadata(approval),
		CreatedAt:   parseApprovalCreatedAt(approval.CreatedAt),
	}
}

func buildApprovalTitle(approval PendingApproval) string {
	actionType := strings.TrimSpace(approval.ActionType)
	target := strings.TrimSpace(approval.TargetResource)
	if actionType == "" {
		actionType = "approval"
	}
	if target == "" {
		return actionType
	}
	return actionType + " - " + target
}

func buildApprovalBody(approval PendingApproval) string {
	reason := strings.TrimSpace(approval.Reason)
	summary := ""
	if approval.AISummary != nil {
		summary = strings.TrimSpace(*approval.AISummary)
	}
	switch {
	case reason != "" && summary != "":
		return reason + "\n\n" + summary
	case reason != "":
		return reason
	case summary != "":
		return summary
	default:
		return "Aprobacion pendiente"
	}
}

func buildApprovalMetadata(approval PendingApproval) json.RawMessage {
	payload, err := json.Marshal(approvalMetadata{
		Source: approvalNotificationSource,
		Approval: approvalMetadataPayload{
			ID:             strings.TrimSpace(approval.ID),
			OrgID:          strings.TrimSpace(approval.OrgID),
			RequestID:      strings.TrimSpace(approval.RequestID),
			ActionType:     strings.TrimSpace(approval.ActionType),
			TargetResource: strings.TrimSpace(approval.TargetResource),
			Reason:         strings.TrimSpace(approval.Reason),
			RiskLevel:      strings.TrimSpace(approval.RiskLevel),
			Status:         strings.TrimSpace(approval.Status),
			AISummary:      approval.AISummary,
			CreatedAt:      strings.TrimSpace(approval.CreatedAt),
			ExpiresAt:      approval.ExpiresAt,
		},
	})
	if err != nil {
		return json.RawMessage(`{"source":"review_approval"}`)
	}
	return json.RawMessage(payload)
}

func parseApprovalCreatedAt(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return parsed.UTC()
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.UTC()
	}
	return time.Time{}
}

func approvalEventToPendingApproval(event ApprovalEvent) PendingApproval {
	return PendingApproval{
		ID:             strings.TrimSpace(event.ApprovalID),
		OrgID:          strings.TrimSpace(event.OrgID),
		RequestID:      strings.TrimSpace(event.RequestID),
		ActionType:     strings.TrimSpace(event.ActionType),
		TargetResource: strings.TrimSpace(event.TargetResource),
		Reason:         strings.TrimSpace(event.Reason),
		RiskLevel:      strings.TrimSpace(event.RiskLevel),
		Status:         strings.TrimSpace(event.Decision),
		AISummary:      event.AISummary,
		CreatedAt:      strings.TrimSpace(event.CreatedAt),
		ExpiresAt:      event.ExpiresAt,
	}
}

func normalizeApprovalEvent(event ApprovalEvent) string {
	switch strings.TrimSpace(event.Event) {
	case approvalEventPending, approvalEventResolved:
		return strings.TrimSpace(event.Event)
	}
	switch strings.ToLower(strings.TrimSpace(event.Decision)) {
	case "pending":
		return approvalEventPending
	case "approved", "rejected", "denied", "deny", "expired":
		return approvalEventResolved
	default:
		return ""
	}
}

func isApprovalNotification(notification coredomain.Notification) bool {
	return strings.TrimSpace(notification.Kind) == approvalNotificationKind &&
		strings.TrimSpace(notification.EntityType) == approvalNotificationEntityType
}
