package procurement

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/devpablocristo/platform/kernels/governance/go/governanceclient"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/core/backend/internal/procurement/usecases/domain"
)

type fakeProcurementRepo struct {
	item domain.ProcurementRequest
	err  error
}

func (f *fakeProcurementRepo) Create(_ context.Context, req domain.ProcurementRequest) (domain.ProcurementRequest, error) {
	f.item = req
	return req, f.err
}

func (f *fakeProcurementRepo) Update(_ context.Context, req domain.ProcurementRequest) (domain.ProcurementRequest, error) {
	f.item = req
	return req, f.err
}

func (f *fakeProcurementRepo) GetByID(_ context.Context, orgID, id uuid.UUID) (domain.ProcurementRequest, error) {
	if f.err != nil {
		return domain.ProcurementRequest{}, f.err
	}
	if f.item.ID != id || f.item.OrgID != orgID {
		return domain.ProcurementRequest{}, ErrNotFound
	}
	return f.item, nil
}

func (f *fakeProcurementRepo) List(_ context.Context, _ uuid.UUID, _ bool, _ int) ([]domain.ProcurementRequest, error) {
	return []domain.ProcurementRequest{f.item}, f.err
}

func (f *fakeProcurementRepo) Delete(context.Context, uuid.UUID, uuid.UUID) error  { return f.err }
func (f *fakeProcurementRepo) Archive(context.Context, uuid.UUID, uuid.UUID) error { return f.err }
func (f *fakeProcurementRepo) Restore(context.Context, uuid.UUID, uuid.UUID) error { return f.err }

type fakeGovernance struct {
	simulateResp governanceclient.SimulateResponse
	simulateErr  error
	submitResp   governanceclient.SubmitResponse
	getResp      governanceclient.RequestSummary
	getStatus    int
	getErr       error
	simulateHits int
	submitHits   int
	getHits      int
}

func (f *fakeGovernance) SimulateRequestForTenant(_ context.Context, _ string, _ governanceclient.SimulateRequestBody) (governanceclient.SimulateResponse, error) {
	f.simulateHits++
	return f.simulateResp, f.simulateErr
}

func (f *fakeGovernance) SubmitRequestForTenant(_ context.Context, _ string, _ string, _ governanceclient.SubmitRequestBody) (governanceclient.SubmitResponse, error) {
	f.submitHits++
	return f.submitResp, nil
}

func (f *fakeGovernance) GetRequestForTenant(_ context.Context, _ string, _ string) (governanceclient.RequestSummary, int, error) {
	f.getHits++
	status := f.getStatus
	if status == 0 {
		status = http.StatusOK
	}
	return f.getResp, status, f.getErr
}

func (f *fakeGovernance) ListPoliciesForTenant(context.Context, string) (int, []byte, error) {
	return 200, []byte(`{"data":[]}`), nil
}

func (f *fakeGovernance) GetPolicyForTenant(context.Context, string, string) (int, []byte, error) {
	return 404, nil, nil
}

func (f *fakeGovernance) CreatePolicyForTenant(context.Context, string, any) (int, []byte, error) {
	return 201, []byte(`{}`), nil
}

func (f *fakeGovernance) UpdatePolicyForTenant(context.Context, string, string, any) (int, []byte, error) {
	return 200, []byte(`{}`), nil
}

func (f *fakeGovernance) DeletePolicyForTenant(context.Context, string, string) (int, error) {
	return 204, nil
}

func TestSumLinesTotal(t *testing.T) {
	t.Parallel()
	got := sumLinesTotal([]domain.RequestLine{
		{Quantity: 2, UnitPriceEstimate: 100},
		{Quantity: 1, UnitPriceEstimate: 50},
	})
	if got != 250 {
		t.Fatalf("sumLinesTotal = %v, want 250", got)
	}
}

func TestBuildPurchaseItemsFromLines(t *testing.T) {
	t.Parallel()
	req := domain.ProcurementRequest{
		ID:    uuid.MustParse("00000000-0000-0000-0000-000000000099"),
		Title: "Test",
		Lines: []domain.RequestLine{
			{Description: "A", Quantity: 2, UnitPriceEstimate: 10},
		},
	}
	items := buildPurchaseItems(req)
	if len(items) != 1 || items[0].Subtotal != 20 {
		t.Fatalf("items = %+v", items)
	}
}

func TestSubmitAllowsViaNexusSimulate(t *testing.T) {
	t.Parallel()
	orgID := uuid.New()
	reqID := uuid.New()
	repo := &fakeProcurementRepo{item: domain.ProcurementRequest{
		ID:             reqID,
		OrgID:          orgID,
		RequesterActor: "owner@example.com",
		Title:          "Compra chica",
		Status:         domain.StatusDraft,
		EstimatedTotal: 500,
		Currency:       "ARS",
	}}
	gov := &fakeGovernance{simulateResp: governanceclient.SimulateResponse{
		Decision:             governanceclient.DecisionAllow,
		Status:               governanceclient.StatusAllowed,
		RiskLevel:            "low",
		DecisionReason:       "auto allowed",
		WouldRequireApproval: false,
	}}
	uc := NewUsecases(repo, gov, nil, nil, nil)

	out, err := uc.Submit(context.Background(), orgID, reqID, "owner@example.com")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if out.Status != domain.StatusApproved {
		t.Fatalf("expected approved, got %s", out.Status)
	}
	if gov.simulateHits != 1 || gov.submitHits != 0 {
		t.Fatalf("expected simulate=1 submit=0, got simulate=%d submit=%d", gov.simulateHits, gov.submitHits)
	}
}

func TestSubmitEscalatesRequireApprovalToNexusSubmit(t *testing.T) {
	t.Parallel()
	orgID := uuid.New()
	reqID := uuid.New()
	repo := &fakeProcurementRepo{item: domain.ProcurementRequest{
		ID:             reqID,
		OrgID:          orgID,
		RequesterActor: "owner@example.com",
		Title:          "Compra grande",
		Status:         domain.StatusDraft,
		EstimatedTotal: 75000,
		Currency:       "ARS",
	}}
	gov := &fakeGovernance{
		simulateResp: governanceclient.SimulateResponse{
			Decision:             governanceclient.DecisionRequireApproval,
			Status:               governanceclient.StatusPendingApproval,
			RiskLevel:            "high",
			DecisionReason:       "threshold exceeded",
			WouldRequireApproval: true,
		},
		submitResp: governanceclient.SubmitResponse{RequestID: "req-nexus-1", Status: governanceclient.StatusPendingApproval},
	}
	uc := NewUsecases(repo, gov, nil, nil, nil)

	out, err := uc.Submit(context.Background(), orgID, reqID, "owner@example.com")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if out.Status != domain.StatusPendingApproval {
		t.Fatalf("expected pending approval, got %s", out.Status)
	}
	if gov.simulateHits != 1 || gov.submitHits != 1 {
		t.Fatalf("expected simulate=1 submit=1, got simulate=%d submit=%d", gov.simulateHits, gov.submitHits)
	}
}

func TestSubmitDenyRejectsWithoutLocalFallback(t *testing.T) {
	t.Parallel()
	orgID := uuid.New()
	reqID := uuid.New()
	repo := &fakeProcurementRepo{item: domain.ProcurementRequest{
		ID:             reqID,
		OrgID:          orgID,
		RequesterActor: "owner@example.com",
		Title:          "Compra bloqueada",
		Status:         domain.StatusDraft,
		EstimatedTotal: 90000,
		Currency:       "ARS",
	}}
	gov := &fakeGovernance{simulateResp: governanceclient.SimulateResponse{
		Decision:       governanceclient.DecisionDeny,
		Status:         governanceclient.StatusDenied,
		RiskLevel:      "critical",
		DecisionReason: "policy denied",
	}}
	uc := NewUsecases(repo, gov, nil, nil, nil)

	out, err := uc.Submit(context.Background(), orgID, reqID, "owner@example.com")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if out.Status != domain.StatusRejected {
		t.Fatalf("expected rejected, got %s", out.Status)
	}
	if gov.submitHits != 0 {
		t.Fatalf("deny must not submit approval request, got submit=%d", gov.submitHits)
	}
}

func TestSubmitFailsClosedWhenNexusSimulateFails(t *testing.T) {
	t.Parallel()
	orgID := uuid.New()
	reqID := uuid.New()
	repo := &fakeProcurementRepo{item: domain.ProcurementRequest{
		ID:             reqID,
		OrgID:          orgID,
		RequesterActor: "owner@example.com",
		Title:          "Compra sin Nexus",
		Status:         domain.StatusDraft,
		EstimatedTotal: 500,
		Currency:       "ARS",
	}}
	gov := &fakeGovernance{simulateErr: errors.New("nexus unavailable")}
	uc := NewUsecases(repo, gov, nil, nil, nil)

	_, err := uc.Submit(context.Background(), orgID, reqID, "owner@example.com")
	if err == nil {
		t.Fatal("expected submit to fail closed when Nexus simulate fails")
	}
	if repo.item.Status != domain.StatusDraft {
		t.Fatalf("expected request to remain draft, got %s", repo.item.Status)
	}
}

func TestApproveRequiresApprovedNexusRequest(t *testing.T) {
	t.Parallel()
	orgID := uuid.New()
	reqID := uuid.New()
	repo := &fakeProcurementRepo{item: domain.ProcurementRequest{
		ID:             reqID,
		OrgID:          orgID,
		RequesterActor: "owner@example.com",
		Title:          "Compra grande",
		Status:         domain.StatusPendingApproval,
		EstimatedTotal: 75000,
		Currency:       "ARS",
		EvaluationJSON: json.RawMessage(`{"nexus_request_id":"req-nexus-1"}`),
	}}
	gov := &fakeGovernance{getResp: governanceclient.RequestSummary{
		ID:     "req-nexus-1",
		Status: governanceclient.StatusApproved,
	}}
	uc := NewUsecases(repo, gov, nil, nil, nil)

	out, err := uc.Approve(context.Background(), orgID, reqID, "owner@example.com")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if out.Status != domain.StatusApproved {
		t.Fatalf("expected approved, got %s", out.Status)
	}
	if gov.getHits != 1 {
		t.Fatalf("expected one nexus verification, got %d", gov.getHits)
	}
}

func TestApproveFailsClosedWithoutNexusRequestID(t *testing.T) {
	t.Parallel()
	orgID := uuid.New()
	reqID := uuid.New()
	repo := &fakeProcurementRepo{item: domain.ProcurementRequest{
		ID:             reqID,
		OrgID:          orgID,
		RequesterActor: "owner@example.com",
		Title:          "Compra grande",
		Status:         domain.StatusPendingApproval,
		EstimatedTotal: 75000,
		Currency:       "ARS",
	}}
	gov := &fakeGovernance{}
	uc := NewUsecases(repo, gov, nil, nil, nil)

	_, err := uc.Approve(context.Background(), orgID, reqID, "owner@example.com")
	if err == nil {
		t.Fatal("expected approve to fail without nexus_request_id")
	}
	if repo.item.Status != domain.StatusPendingApproval {
		t.Fatalf("expected request to remain pending approval, got %s", repo.item.Status)
	}
	if gov.getHits != 0 {
		t.Fatalf("expected no nexus fetch without request id, got %d", gov.getHits)
	}
}

func TestApproveFailsClosedWhenNexusStillPending(t *testing.T) {
	t.Parallel()
	orgID := uuid.New()
	reqID := uuid.New()
	repo := &fakeProcurementRepo{item: domain.ProcurementRequest{
		ID:             reqID,
		OrgID:          orgID,
		RequesterActor: "owner@example.com",
		Title:          "Compra grande",
		Status:         domain.StatusPendingApproval,
		EstimatedTotal: 75000,
		Currency:       "ARS",
		EvaluationJSON: json.RawMessage(`{"nexus_request_id":"req-nexus-1"}`),
	}}
	gov := &fakeGovernance{getResp: governanceclient.RequestSummary{
		ID:     "req-nexus-1",
		Status: governanceclient.StatusPendingApproval,
	}}
	uc := NewUsecases(repo, gov, nil, nil, nil)

	_, err := uc.Approve(context.Background(), orgID, reqID, "owner@example.com")
	if err == nil {
		t.Fatal("expected approve to fail while Nexus is still pending")
	}
	if repo.item.Status != domain.StatusPendingApproval {
		t.Fatalf("expected request to remain pending approval, got %s", repo.item.Status)
	}
}

func TestRejectRequiresRejectedNexusRequest(t *testing.T) {
	t.Parallel()
	orgID := uuid.New()
	reqID := uuid.New()
	repo := &fakeProcurementRepo{item: domain.ProcurementRequest{
		ID:             reqID,
		OrgID:          orgID,
		RequesterActor: "owner@example.com",
		Title:          "Compra rechazada",
		Status:         domain.StatusPendingApproval,
		EstimatedTotal: 75000,
		Currency:       "ARS",
		EvaluationJSON: json.RawMessage(`{"nexus_request_id":"req-nexus-1"}`),
	}}
	gov := &fakeGovernance{getResp: governanceclient.RequestSummary{
		ID:     "req-nexus-1",
		Status: governanceclient.StatusRejected,
	}}
	uc := NewUsecases(repo, gov, nil, nil, nil)

	out, err := uc.Reject(context.Background(), orgID, reqID, "owner@example.com")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if out.Status != domain.StatusRejected {
		t.Fatalf("expected rejected, got %s", out.Status)
	}
}
