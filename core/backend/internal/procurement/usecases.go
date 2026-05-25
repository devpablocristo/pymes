package procurement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/platform/errors/go/domainerr"
	"github.com/devpablocristo/platform/kernels/governance/go/governanceclient"

	"github.com/devpablocristo/pymes/core/backend/internal/procurement/usecases/domain"
	"github.com/devpablocristo/pymes/core/backend/internal/purchases"
	purchasesdomain "github.com/devpablocristo/pymes/core/backend/internal/purchases/usecases/domain"
)

type auditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type timelinePort interface {
	RecordEvent(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID, eventType, title, description, actor string, metadata map[string]any) error
}

type purchasesPort interface {
	Create(ctx context.Context, in purchases.CreateInput) (purchasesdomain.Purchase, error)
}

type repositoryPort interface {
	Create(ctx context.Context, req domain.ProcurementRequest) (domain.ProcurementRequest, error)
	Update(ctx context.Context, req domain.ProcurementRequest) (domain.ProcurementRequest, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.ProcurementRequest, error)
	List(ctx context.Context, orgID uuid.UUID, includeArchived bool, limit int) ([]domain.ProcurementRequest, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	Archive(ctx context.Context, orgID, id uuid.UUID) error
	Restore(ctx context.Context, orgID, id uuid.UUID) error
}

// governancePort es la superficie del client de Nexus que procurement consume.
// Contrato HTTP: Pymes nunca evalúa policies en proceso. Nexus es source of
// truth y las policies viven en Nexus por tenant.
type governancePort interface {
	SimulateRequestForTenant(ctx context.Context, orgID string, body governanceclient.SimulateRequestBody) (governanceclient.SimulateResponse, error)
	SubmitRequestForTenant(ctx context.Context, orgID, idempotencyKey string, body governanceclient.SubmitRequestBody) (governanceclient.SubmitResponse, error)
	GetRequestForTenant(ctx context.Context, orgID, id string) (governanceclient.RequestSummary, int, error)
	ListPoliciesForTenant(ctx context.Context, orgID string) (int, []byte, error)
	GetPolicyForTenant(ctx context.Context, orgID, id string) (int, []byte, error)
	CreatePolicyForTenant(ctx context.Context, orgID string, body any) (int, []byte, error)
	UpdatePolicyForTenant(ctx context.Context, orgID, id string, body any) (int, []byte, error)
	DeletePolicyForTenant(ctx context.Context, orgID, id string) (int, error)
}

type Usecases struct {
	repo       repositoryPort
	governance governancePort
	purchases  purchasesPort
	audit      auditPort
	timeline   timelinePort
	webhooks   webhookPort
}

// NewUsecases construye el módulo. governance es OBLIGATORIO: sin él,
// procurement no puede decidir nada (Pymes no decide gobernanza local).
// Pasar nil hace fail-fast en boot vía panic — preferible a corromper estado.
func NewUsecases(
	repo repositoryPort,
	governance governancePort,
	purchases purchasesPort,
	audit auditPort,
	timeline timelinePort,
	opts ...Option,
) *Usecases {
	if governance == nil {
		panic("procurement: governance client is required (set GOVERNANCE_URL / GOVERNANCE_API_KEY)")
	}
	u := &Usecases{
		repo:       repo,
		governance: governance,
		purchases:  purchases,
		audit:      audit,
		timeline:   timeline,
	}
	for _, o := range opts {
		if o != nil {
			o(u)
		}
	}
	return u
}

type CreateInput struct {
	OrgID          uuid.UUID
	Actor          string
	Title          string
	Description    string
	Category       string
	EstimatedTotal float64
	Currency       string
	Lines          []domain.RequestLine
}

func (u *Usecases) Create(ctx context.Context, in CreateInput) (domain.ProcurementRequest, error) {
	if strings.TrimSpace(in.Title) == "" {
		return domain.ProcurementRequest{}, domainerr.Validation("title is required")
	}
	if in.OrgID == uuid.Nil {
		return domain.ProcurementRequest{}, domainerr.Validation("org_id is required")
	}
	actor := strings.TrimSpace(in.Actor)
	if actor == "" {
		return domain.ProcurementRequest{}, domainerr.Validation("actor is required")
	}
	now := time.Now()
	total := in.EstimatedTotal
	if len(in.Lines) > 0 {
		total = sumLinesTotal(in.Lines)
	}
	req := domain.ProcurementRequest{
		ID:             uuid.New(),
		OrgID:          in.OrgID,
		RequesterActor: actor,
		Title:          strings.TrimSpace(in.Title),
		Description:    strings.TrimSpace(in.Description),
		Category:       strings.TrimSpace(in.Category),
		Status:         domain.StatusDraft,
		EstimatedTotal: total,
		Currency:       defaultString(strings.TrimSpace(in.Currency), "ARS"),
		Lines:          normalizeLines(in.Lines, uuid.Nil),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	out, err := u.repo.Create(ctx, req)
	if err != nil {
		return domain.ProcurementRequest{}, err
	}
	u.logAudit(ctx, in.OrgID, actor, "procurement_request.created", out.ID.String(), map[string]any{"title": out.Title})
	u.emitWebhook(ctx, in.OrgID, "procurement_request.created", map[string]any{
		"procurement_request_id": out.ID.String(),
		"title":                  out.Title,
		"status":                 string(out.Status),
	})
	return out, nil
}

type UpdateInput struct {
	OrgID          uuid.UUID
	ID             uuid.UUID
	Actor          string
	Title          string
	Description    string
	Category       string
	EstimatedTotal float64
	Currency       string
	Lines          []domain.RequestLine
}

func (u *Usecases) Update(ctx context.Context, in UpdateInput) (domain.ProcurementRequest, error) {
	cur, err := u.repo.GetByID(ctx, in.OrgID, in.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.ProcurementRequest{}, domainerr.NotFoundf("procurement_request", in.ID.String())
		}
		return domain.ProcurementRequest{}, err
	}
	if cur.Status != domain.StatusDraft {
		return domain.ProcurementRequest{}, domainerr.BusinessRule("only draft procurement requests can be updated")
	}
	if strings.TrimSpace(in.Title) == "" {
		return domain.ProcurementRequest{}, domainerr.Validation("title is required")
	}
	total := in.EstimatedTotal
	lines := normalizeLines(in.Lines, in.ID)
	if len(lines) > 0 {
		total = sumLinesTotal(lines)
	}
	cur.Title = strings.TrimSpace(in.Title)
	cur.Description = strings.TrimSpace(in.Description)
	cur.Category = strings.TrimSpace(in.Category)
	cur.EstimatedTotal = total
	cur.Currency = defaultString(strings.TrimSpace(in.Currency), cur.Currency)
	cur.Lines = lines
	cur.UpdatedAt = time.Now()
	out, err := u.repo.Update(ctx, cur)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.ProcurementRequest{}, domainerr.NotFoundf("procurement_request", in.ID.String())
		}
		if errors.Is(err, ErrArchived) {
			return domain.ProcurementRequest{}, domainerr.BusinessRule("procurement request is archived")
		}
		return domain.ProcurementRequest{}, err
	}
	u.logAudit(ctx, in.OrgID, in.Actor, "procurement_request.updated", out.ID.String(), nil)
	u.emitWebhook(ctx, in.OrgID, "procurement_request.updated", map[string]any{
		"procurement_request_id": out.ID.String(),
		"status":                 string(out.Status),
	})
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.ProcurementRequest, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.ProcurementRequest{}, domainerr.NotFoundf("procurement_request", id.String())
		}
		return domain.ProcurementRequest{}, err
	}
	return out, nil
}

func (u *Usecases) List(ctx context.Context, orgID uuid.UUID, archived bool, limit int) ([]domain.ProcurementRequest, error) {
	return u.repo.List(ctx, orgID, archived, limit)
}

func (u *Usecases) Delete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Delete(ctx, orgID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return domainerr.NotFoundf("procurement_request", id.String())
		}
		return err
	}
	u.logAudit(ctx, orgID, actor, "procurement_request.deleted", id.String(), nil)
	u.emitWebhook(ctx, orgID, "procurement_request.deleted", map[string]any{"procurement_request_id": id.String()})
	return nil
}

func (u *Usecases) Archive(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Archive(ctx, orgID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return domainerr.NotFoundf("procurement_request", id.String())
		}
		return err
	}
	u.logAudit(ctx, orgID, actor, "procurement_request.archived", id.String(), nil)
	u.emitWebhook(ctx, orgID, "procurement_request.archived", map[string]any{"procurement_request_id": id.String()})
	return nil
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return domainerr.NotFoundf("procurement_request", id.String())
		}
		return err
	}
	u.logAudit(ctx, orgID, actor, "procurement_request.restored", id.String(), nil)
	u.emitWebhook(ctx, orgID, "procurement_request.restored", map[string]any{"procurement_request_id": id.String()})
	return nil
}

// procurementSubmitParams arma los params de evidencia que viajan a Nexus.
// El tenant efectivo viaja por el adapter de governance; el body queda como
// evidencia de negocio para policy/eval.
func procurementSubmitParams(req domain.ProcurementRequest, total float64) map[string]any {
	return map[string]any{
		"estimated_total": total,
		"category":        req.Category,
		"currency":        req.Currency,
		"org_id":          req.OrgID.String(),
	}
}

// Submit envía el procurement request a Nexus para evaluación. Si Nexus
// permite (allow), se crea el purchase. Si requiere aprobación humana, Pymes
// escala con SubmitRequest (persistente en Nexus) y el procurement queda en
// PendingApproval con el nexus_request_id guardado en EvaluationJSON. Si
// deniega, se rechaza.
//
// El motor de policies vive 100% en Nexus — Pymes ya no embebe nada.
func (u *Usecases) Submit(ctx context.Context, orgID, id uuid.UUID, actor string) (domain.ProcurementRequest, error) {
	req, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.ProcurementRequest{}, domainerr.NotFoundf("procurement_request", id.String())
		}
		return domain.ProcurementRequest{}, err
	}
	if req.Status != domain.StatusDraft {
		return domain.ProcurementRequest{}, domainerr.BusinessRule("only draft requests can be submitted")
	}
	total := req.EstimatedTotal
	if len(req.Lines) > 0 {
		total = sumLinesTotal(req.Lines)
	}
	req.EstimatedTotal = total

	body := governanceclient.SimulateRequestBody{
		RequesterType:  "human",
		RequesterID:    actor,
		RequesterName:  actor,
		ActionType:     "procurement.submit",
		TargetSystem:   "pymes",
		TargetResource: "procurement_request",
		Params:         procurementSubmitParams(req, total),
	}
	sim, err := u.governance.SimulateRequestForTenant(ctx, orgID.String(), body)
	if err != nil {
		return domain.ProcurementRequest{}, fmt.Errorf("nexus simulate procurement.submit: %w", err)
	}

	evalRecord := map[string]any{
		"decision":               sim.Decision,
		"risk_level":             sim.RiskLevel,
		"decision_reason":        sim.DecisionReason,
		"status":                 sim.Status,
		"would_require_approval": sim.WouldRequireApproval,
		"policy_matched":         sim.PolicyMatched,
		"evaluated_at":           time.Now().UTC().Format(time.RFC3339),
		"source":                 "simulate",
	}

	switch sim.Decision {
	case governanceclient.DecisionDeny:
		req.Status = domain.StatusRejected
	case governanceclient.DecisionAllow:
		req.Status = domain.StatusApproved
	case governanceclient.DecisionRequireApproval:
		// Escalamos a SubmitRequest para crear el request persistente en Nexus
		// (con su approval row + audit). El procurement queda esperando que un
		// humano apruebe en consola Nexus; el FSM se cierra cuando alguien
		// llama Approve/Reject acá (que consultará el status en Nexus).
		submitBody := governanceclient.SubmitRequestBody{
			RequesterType:  body.RequesterType,
			RequesterID:    body.RequesterID,
			RequesterName:  body.RequesterName,
			ActionType:     body.ActionType,
			TargetSystem:   body.TargetSystem,
			TargetResource: body.TargetResource,
			Params:         body.Params,
			Reason:         fmt.Sprintf("procurement request %s", req.ID),
			Context:        "core procurement.submit",
		}
		idemKey := fmt.Sprintf("procurement-%s-%s", orgID.String(), req.ID)
		submitResp, subErr := u.governance.SubmitRequestForTenant(ctx, orgID.String(), idemKey, submitBody)
		if subErr != nil {
			return domain.ProcurementRequest{}, fmt.Errorf("nexus submit procurement.submit (require_approval escalation): %w", subErr)
		}
		req.Status = domain.StatusPendingApproval
		evalRecord["nexus_request_id"] = submitResp.RequestID
		evalRecord["nexus_status"] = submitResp.Status
		evalRecord["source"] = "submit_escalated"
	default:
		req.Status = domain.StatusPendingApproval
		evalRecord["unknown_decision"] = sim.Decision
	}

	if evalBytes, mErr := json.Marshal(evalRecord); mErr == nil {
		req.EvaluationJSON = evalBytes
	}
	req.UpdatedAt = time.Now()

	out, err := u.repo.Update(ctx, req)
	if err != nil {
		return domain.ProcurementRequest{}, err
	}

	if sim.Decision == governanceclient.DecisionAllow && u.purchases != nil {
		pu, perr := u.createPurchaseFromRequest(ctx, out, actor)
		if perr != nil {
			slog.Error("procurement create purchase after approval", "error", perr, "request_id", out.ID)
		} else {
			out.PurchaseID = &pu.ID
			out, err = u.repo.Update(ctx, out)
			if err != nil {
				return domain.ProcurementRequest{}, err
			}
		}
	}

	u.logAudit(ctx, orgID, actor, "procurement_request.submitted", out.ID.String(), map[string]any{"decision": sim.Decision})
	if u.timeline != nil {
		_ = u.timeline.RecordEvent(ctx, orgID, "procurement_request", out.ID, "procurement_request.submitted",
			"Solicitud de compra enviada", out.Title, actor, map[string]any{"decision": sim.Decision})
	}
	u.emitWebhook(ctx, orgID, "procurement_request.submitted", map[string]any{
		"procurement_request_id": out.ID.String(),
		"decision":               sim.Decision,
		"status":                 string(out.Status),
		"purchase_id":            nullableUUIDPtr(out.PurchaseID),
	})
	return out, nil
}

// Approve finaliza un procurement request en Pendiente solo si Nexus ya
// registró la aprobación humana. Pymes refleja el estado y crea el purchase;
// Nexus sigue siendo source of truth de la decisión.
func (u *Usecases) Approve(ctx context.Context, orgID, id uuid.UUID, actor string) (domain.ProcurementRequest, error) {
	req, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.ProcurementRequest{}, domainerr.NotFoundf("procurement_request", id.String())
		}
		return domain.ProcurementRequest{}, err
	}
	if req.Status != domain.StatusPendingApproval {
		return domain.ProcurementRequest{}, domainerr.BusinessRule("only pending requests can be approved")
	}
	verified, err := u.verifyNexusProcurementDecision(ctx, req, governanceclient.StatusApproved)
	if err != nil {
		return domain.ProcurementRequest{}, err
	}
	req.Status = domain.StatusApproved
	req.EvaluationJSON = mergeEvaluationJSON(req.EvaluationJSON, map[string]any{
		"nexus_verified_status": verified.Status,
		"nexus_verified_at":     time.Now().UTC().Format(time.RFC3339),
		"nexus_verified_by":     actor,
	})
	req.UpdatedAt = time.Now()
	out, err := u.repo.Update(ctx, req)
	if err != nil {
		return domain.ProcurementRequest{}, err
	}
	if u.purchases != nil && out.PurchaseID == nil {
		pu, perr := u.createPurchaseFromRequest(ctx, out, actor)
		if perr != nil {
			slog.Error("procurement create purchase on approve", "error", perr, "request_id", out.ID)
		} else {
			out.PurchaseID = &pu.ID
			out, err = u.repo.Update(ctx, out)
			if err != nil {
				return domain.ProcurementRequest{}, err
			}
		}
	}
	u.logAudit(ctx, orgID, actor, "procurement_request.approved", out.ID.String(), nil)
	u.emitWebhook(ctx, orgID, "procurement_request.approved", map[string]any{
		"procurement_request_id": out.ID.String(),
		"status":                 string(out.Status),
		"purchase_id":            nullableUUIDPtr(out.PurchaseID),
	})
	return out, nil
}

func (u *Usecases) Reject(ctx context.Context, orgID, id uuid.UUID, actor string) (domain.ProcurementRequest, error) {
	req, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.ProcurementRequest{}, domainerr.NotFoundf("procurement_request", id.String())
		}
		return domain.ProcurementRequest{}, err
	}
	if req.Status != domain.StatusPendingApproval {
		return domain.ProcurementRequest{}, domainerr.BusinessRule("only pending requests can be rejected")
	}
	verified, err := u.verifyNexusProcurementDecision(ctx, req, governanceclient.StatusRejected)
	if err != nil {
		return domain.ProcurementRequest{}, err
	}
	req.Status = domain.StatusRejected
	req.EvaluationJSON = mergeEvaluationJSON(req.EvaluationJSON, map[string]any{
		"nexus_verified_status": verified.Status,
		"nexus_verified_at":     time.Now().UTC().Format(time.RFC3339),
		"nexus_verified_by":     actor,
	})
	req.UpdatedAt = time.Now()
	out, err := u.repo.Update(ctx, req)
	if err != nil {
		return domain.ProcurementRequest{}, err
	}
	u.logAudit(ctx, orgID, actor, "procurement_request.rejected", out.ID.String(), nil)
	u.emitWebhook(ctx, orgID, "procurement_request.rejected", map[string]any{
		"procurement_request_id": out.ID.String(),
		"status":                 string(out.Status),
	})
	return out, nil
}

func (u *Usecases) verifyNexusProcurementDecision(ctx context.Context, req domain.ProcurementRequest, wantStatus string) (governanceclient.RequestSummary, error) {
	nexusRequestID := nexusRequestIDFromEvaluation(req.EvaluationJSON)
	if nexusRequestID == "" {
		return governanceclient.RequestSummary{}, domainerr.BusinessRule("procurement request has no nexus_request_id to verify")
	}
	summary, status, err := u.governance.GetRequestForTenant(ctx, req.OrgID.String(), nexusRequestID)
	if err != nil {
		return governanceclient.RequestSummary{}, fmt.Errorf("nexus get procurement request %s: %w", nexusRequestID, err)
	}
	if status == http.StatusNotFound {
		return governanceclient.RequestSummary{}, domainerr.BusinessRule("nexus request not found")
	}
	if status != http.StatusOK {
		return governanceclient.RequestSummary{}, fmt.Errorf("nexus get procurement request %s: status %d", nexusRequestID, status)
	}
	if summary.Status != wantStatus {
		return governanceclient.RequestSummary{}, domainerr.BusinessRule(fmt.Sprintf("nexus request status must be %s, got %s", wantStatus, summary.Status))
	}
	return summary, nil
}

func nexusRequestIDFromEvaluation(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	if v, ok := payload["nexus_request_id"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func mergeEvaluationJSON(raw json.RawMessage, values map[string]any) json.RawMessage {
	payload := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &payload)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	for k, v := range values {
		payload[k] = v
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return raw
	}
	return out
}

func (u *Usecases) createPurchaseFromRequest(ctx context.Context, req domain.ProcurementRequest, actor string) (purchasesdomain.Purchase, error) {
	items := buildPurchaseItems(req)
	if len(items) == 0 {
		return purchasesdomain.Purchase{}, domainerr.BusinessRule("procurement request has no lines for purchase")
	}
	notes := fmt.Sprintf("Generado desde solicitud interna %s", req.ID.String())
	return u.purchases.Create(ctx, purchases.CreateInput{
		OrgID:         req.OrgID,
		SupplierName:  "Pendiente (solicitud interna)",
		Status:        "draft",
		PaymentStatus: "pending",
		Notes:         notes,
		CreatedBy:     actor,
		Items:         items,
	})
}

func buildPurchaseItems(req domain.ProcurementRequest) []purchasesdomain.PurchaseItem {
	if len(req.Lines) == 0 {
		if req.EstimatedTotal <= 0 {
			return nil
		}
		return []purchasesdomain.PurchaseItem{
			{
				ID:          uuid.New(),
				Description: strings.TrimSpace(req.Title),
				Quantity:    1,
				UnitCost:    req.EstimatedTotal,
				TaxRate:     0,
				Subtotal:    req.EstimatedTotal,
				SortOrder:   0,
			},
		}
	}
	out := make([]purchasesdomain.PurchaseItem, 0, len(req.Lines))
	for i, line := range req.Lines {
		desc := strings.TrimSpace(line.Description)
		if desc == "" {
			desc = fmt.Sprintf("Ítem %d", i+1)
		}
		qty := line.Quantity
		if qty <= 0 {
			qty = 1
		}
		uc := line.UnitPriceEstimate
		if uc < 0 {
			uc = 0
		}
		sub := qty * uc
		out = append(out, purchasesdomain.PurchaseItem{
			ID:          uuid.New(),
			ProductID:   line.ProductID,
			Description: desc,
			Quantity:    qty,
			UnitCost:    uc,
			TaxRate:     0,
			Subtotal:    sub,
			SortOrder:   i,
		})
	}
	return out
}

func (u *Usecases) logAudit(ctx context.Context, orgID uuid.UUID, actor, action, resourceID string, payload map[string]any) {
	if u.audit == nil {
		return
	}
	u.audit.Log(ctx, orgID.String(), actor, action, "procurement_request", resourceID, payload)
}

func (u *Usecases) emitWebhook(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) {
	if u.webhooks == nil {
		return
	}
	_ = u.webhooks.Enqueue(ctx, orgID, eventType, payload)
}

func nullableUUIDPtr(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func sumLinesTotal(lines []domain.RequestLine) float64 {
	var t float64
	for _, l := range lines {
		q := l.Quantity
		if q <= 0 {
			q = 1
		}
		t += q * l.UnitPriceEstimate
	}
	return t
}

func normalizeLines(lines []domain.RequestLine, requestID uuid.UUID) []domain.RequestLine {
	out := make([]domain.RequestLine, 0, len(lines))
	for i, l := range lines {
		if l.ID == uuid.Nil {
			l.ID = uuid.New()
		}
		l.RequestID = requestID
		l.SortOrder = i
		if l.Quantity <= 0 {
			l.Quantity = 1
		}
		if l.UnitPriceEstimate < 0 {
			l.UnitPriceEstimate = 0
		}
		out = append(out, l)
	}
	return out
}
