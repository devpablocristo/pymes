package reviewproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/devpablocristo/core/governance/go/reviewclient"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/inappnotifications"
	reviewdto "github.com/devpablocristo/pymes/pymes-core/backend/internal/reviewproxy/handler/dto"
)

type pendingApprovalSourceClient interface {
	ListPendingApprovals(ctx context.Context) (int, []byte, error)
}

type pendingApprovalListPayload struct {
	Data      []reviewdto.ApprovalResponse `json:"data"`
	Approvals []reviewdto.ApprovalResponse `json:"approvals"`
}

type PendingApprovalSource struct {
	client pendingApprovalSourceClient
}

func NewPendingApprovalSource(client pendingApprovalSourceClient) *PendingApprovalSource {
	return &PendingApprovalSource{client: client}
}

func (s *PendingApprovalSource) ListPendingApprovals(ctx context.Context) ([]inappnotifications.PendingApproval, error) {
	status, data, err := s.client.ListPendingApprovals(ctx)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("review pending approvals: status %d body %s", status, reviewclient.ParseErrorBody(data))
	}

	var payload pendingApprovalListPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode review approvals: %w", err)
	}
	approvals := payload.Data
	if len(approvals) == 0 && len(payload.Approvals) > 0 {
		approvals = payload.Approvals
	}

	out := make([]inappnotifications.PendingApproval, 0, len(approvals))
	for _, approval := range approvals {
		out = append(out, inappnotifications.PendingApproval{
			ID:             approval.ID,
			OrgID:          approval.OrgID,
			RequestID:      approval.RequestID,
			ActionType:     approval.ActionType,
			TargetResource: approval.TargetResource,
			Reason:         approval.Reason,
			RiskLevel:      approval.RiskLevel,
			Status:         approval.Status,
			AISummary:      approval.AISummary,
			CreatedAt:      approval.CreatedAt,
			ExpiresAt:      approval.ExpiresAt,
		})
	}
	return out, nil
}

var _ inappnotifications.ApprovalSource = (*PendingApprovalSource)(nil)
