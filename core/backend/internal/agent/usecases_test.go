package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/devpablocristo/platform/kernels/governance/go/governanceclient"
	"github.com/google/uuid"
)

type fakeAgentRepo struct {
	confirmation Confirmation
	markHits     int
}

func (f *fakeAgentRepo) CreateConfirmation(_ context.Context, in Confirmation) (Confirmation, error) {
	f.confirmation = in
	return in, nil
}

func (f *fakeAgentRepo) GetConfirmation(_ context.Context, orgID uuid.UUID, id uuid.UUID) (Confirmation, error) {
	if f.confirmation.ID != id || f.confirmation.OrgID != orgID {
		return Confirmation{}, errTestNotFound{}
	}
	return f.confirmation, nil
}

func (f *fakeAgentRepo) MarkConfirmationUsed(_ context.Context, orgID uuid.UUID, id uuid.UUID) error {
	if f.confirmation.ID != id || f.confirmation.OrgID != orgID || f.confirmation.Status != "pending" {
		return errTestNotFound{}
	}
	now := time.Now().UTC()
	f.confirmation.Status = "used"
	f.confirmation.UsedAt = &now
	f.markHits++
	return nil
}

func (f *fakeAgentRepo) GetIdempotencyRecord(context.Context, uuid.UUID, string, string, string) (IdempotencyRecord, bool, error) {
	return IdempotencyRecord{}, false, nil
}

func (f *fakeAgentRepo) SaveIdempotencyRecord(context.Context, IdempotencyRecord) error { return nil }

func (f *fakeAgentRepo) ListAgentEvents(context.Context, uuid.UUID, int, string, string) ([]AgentEvent, error) {
	return nil, nil
}

type fakeAgentGovernance struct {
	submitResp governanceclient.SubmitResponse
	getResp    governanceclient.RequestSummary
	getStatus  int
	body       governanceclient.SubmitRequestBody
}

func (f *fakeAgentGovernance) SubmitRequestForTenant(_ context.Context, _ string, _ string, body governanceclient.SubmitRequestBody) (governanceclient.SubmitResponse, error) {
	f.body = body
	return f.submitResp, nil
}

func (f *fakeAgentGovernance) GetRequestForTenant(context.Context, string, string) (governanceclient.RequestSummary, int, error) {
	return f.getResp, f.getStatus, nil
}

type errTestNotFound struct{}

func (errTestNotFound) Error() string { return "not found" }

func TestExecuteConsumesConfirmationWhenReviewIsSubmitted(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	payload := json.RawMessage(`{"amount":100}`)
	payloadHash, err := PayloadHashFromRaw(payload)
	if err != nil {
		t.Fatalf("hash payload: %v", err)
	}
	confirmationID := uuid.New()
	repo := &fakeAgentRepo{confirmation: Confirmation{
		ID:           confirmationID,
		OrgID:        orgID,
		Actor:        "owner@example.com",
		CapabilityID: "pymes.sales.create",
		PayloadHash:  payloadHash,
		Status:       "pending",
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
	}}
	gov := &fakeAgentGovernance{submitResp: governanceclient.SubmitResponse{
		RequestID: "req-1",
		Decision:  governanceclient.DecisionRequireApproval,
		Status:    governanceclient.StatusPendingApproval,
	}}
	uc := NewUsecases(repo, gov, nil)

	result, err := uc.Execute(context.Background(), ExecuteInput{
		Auth: ActorContext{
			OrgID:      orgID.String(),
			Actor:      "owner@example.com",
			AuthMethod: "session",
		},
		CapabilityID:   "pymes.sales.create",
		Payload:        payload,
		ConfirmationID: confirmationID.String(),
		IdempotencyKey: "idem-1",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.StatusCode != 202 {
		t.Fatalf("status = %d, want 202", result.StatusCode)
	}
	if repo.markHits != 1 || repo.confirmation.Status != "used" {
		t.Fatalf("confirmation was not consumed: hits=%d status=%s", repo.markHits, repo.confirmation.Status)
	}
	binding, ok := gov.body.Params["action_binding"].(map[string]any)
	if !ok {
		t.Fatalf("expected action_binding in governance params: %+v", gov.body.Params)
	}
	if binding["schema_version"] != "tool_intent.v1" || binding["org_id"] != orgID.String() || binding["capability_id"] != "pymes.sales.create" {
		t.Fatalf("unexpected strict action binding %+v", binding)
	}
}

func TestExecuteRequiresApprovedReviewRequest(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	payload := json.RawMessage(`{"amount":100}`)
	payloadHash, err := PayloadHashFromRaw(payload)
	if err != nil {
		t.Fatalf("hash payload: %v", err)
	}
	confirmationID := uuid.New()
	repo := &fakeAgentRepo{confirmation: Confirmation{
		ID:           confirmationID,
		OrgID:        orgID,
		Actor:        "owner@example.com",
		CapabilityID: "pymes.sales.create",
		PayloadHash:  payloadHash,
		Status:       "pending",
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
	}}
	uc := NewUsecases(repo, &fakeAgentGovernance{
		getStatus: 200,
		getResp: governanceclient.RequestSummary{
			ID:       "req-1",
			Decision: governanceclient.DecisionRequireApproval,
			Status:   governanceclient.StatusPendingApproval,
		},
	}, nil)

	result, err := uc.Execute(context.Background(), ExecuteInput{
		Auth: ActorContext{
			OrgID:      orgID.String(),
			Actor:      "owner@example.com",
			AuthMethod: "session",
		},
		CapabilityID:    "pymes.sales.create",
		Payload:         payload,
		ConfirmationID:  confirmationID.String(),
		ReviewRequestID: "req-1",
		IdempotencyKey:  "idem-1",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.StatusCode != 202 {
		t.Fatalf("status = %d, want 202", result.StatusCode)
	}
	if repo.markHits != 0 {
		t.Fatalf("confirmation should remain pending until review is approved")
	}
}
