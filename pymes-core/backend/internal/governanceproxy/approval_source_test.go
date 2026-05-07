package governanceproxy

import (
	"context"
	"testing"
)

type stubPendingApprovalSourceClient struct {
	status int
	data   []byte
	err    error
}

func (s stubPendingApprovalSourceClient) ListPendingApprovals(context.Context) (int, []byte, error) {
	return s.status, s.data, s.err
}

func TestPendingApprovalSourceListPendingApprovalsDecodesGovernanceDataShape(t *testing.T) {
	t.Parallel()

	source := NewPendingApprovalSource(stubPendingApprovalSourceClient{
		status: 200,
		data: []byte(`{
			"data": [
				{
					"id": "approval-1",
					"tenant_id": "00000000-0000-0000-0000-000000000001",
					"request_id": "request-1",
					"status": "pending",
					"created_at": "2026-04-01T17:30:00Z",
					"expires_at": "2026-04-01T18:30:00Z"
				}
			]
		}`),
	})

	approvals, err := source.ListPendingApprovals(context.Background())
	if err != nil {
		t.Fatalf("ListPendingApprovals() error = %v", err)
	}
	if len(approvals) != 1 {
		t.Fatalf("expected 1 approval, got %d", len(approvals))
	}
	if approvals[0].ID != "approval-1" {
		t.Fatalf("expected approval id approval-1, got %q", approvals[0].ID)
	}
	if approvals[0].TenantID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("expected org id from governance payload, got %q", approvals[0].TenantID)
	}
	if approvals[0].RequestID != "request-1" {
		t.Fatalf("expected request id request-1, got %q", approvals[0].RequestID)
	}
}

func TestPendingApprovalSourceListPendingApprovalsSupportsLegacyProxyShape(t *testing.T) {
	t.Parallel()

	source := NewPendingApprovalSource(stubPendingApprovalSourceClient{
		status: 200,
		data: []byte(`{
			"approvals": [
				{
					"id": "approval-imported",
					"tenant_id": "00000000-0000-0000-0000-000000000001",
					"request_id": "request-imported",
					"status": "pending",
					"created_at": "2026-04-01T17:30:00Z"
				}
			],
			"total": 1
		}`),
	})

	approvals, err := source.ListPendingApprovals(context.Background())
	if err != nil {
		t.Fatalf("ListPendingApprovals() error = %v", err)
	}
	if len(approvals) != 1 {
		t.Fatalf("expected 1 approval, got %d", len(approvals))
	}
	if approvals[0].ID != "approval-imported" {
		t.Fatalf("expected imported approval id, got %q", approvals[0].ID)
	}
}
