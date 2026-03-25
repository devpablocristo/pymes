package procurement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	govdecision "github.com/devpablocristo/core/governance/go/decision"
	kerneldomain "github.com/devpablocristo/core/governance/go/kernel/usecases/domain"
	"github.com/devpablocristo/core/governance/go/risk"
	"github.com/devpablocristo/core/errors/go/domainerr"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/procurement/repository/models"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/procurement/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/purchases"
	purchasesdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/purchases/usecases/domain"
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
	ListPolicies(ctx context.Context, orgID uuid.UUID) ([]models.ProcurementPolicy, error)
	GetPolicyByID(ctx context.Context, orgID, id uuid.UUID) (domain.ProcurementPolicy, error)
	SavePolicy(ctx context.Context, p domain.ProcurementPolicy) (domain.ProcurementPolicy, error)
	DeletePolicy(ctx context.Context, orgID, id uuid.UUID) error
}

type Usecases struct {
	repo      repositoryPort
	engine    *govdecision.Engine
	purchases purchasesPort
	audit     auditPort
	timeline  timelinePort
	webhooks  webhookPort
}

func NewUsecases(
	repo repositoryPort,
	engine *govdecision.Engine,
	purchases purchasesPort,
	audit auditPort,
	timeline timelinePort,
	opts ...Option,
) *Usecases {
	u := &Usecases{
		repo:      repo,
		engine:    engine,
		purchases: purchases,
		audit:     audit,
		timeline:  timeline,
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

	kernelReq := kerneldomain.Request{
		ID:        uuid.New().String(),
		Subject:   kerneldomain.Subject{Type: kerneldomain.RequesterTypeHuman, ID: actor, Name: actor},
		Action:    "procurement.submit",
		Target:    kerneldomain.Target{System: "pymes", Resource: "procurement_request"},
		Params:    map[string]any{"estimated_total": total, "category": req.Category, "currency": req.Currency},
		CreatedAt: time.Now().UTC(),
	}

	policyRows, err := u.repo.ListPolicies(ctx, orgID)
	if err != nil {
		return domain.ProcurementRequest{}, err
	}
	policies := mapDBPoliciesToKernel(policyRows)
	if len(policies) == 0 {
		policies = defaultKernelPolicies()
	}

	eval, err := u.engine.Evaluate(govdecision.Input{
		Request:  kernelReq,
		Policies: policies,
		History:  risk.History{},
		Now:      time.Now().UTC(),
	})
	if err != nil {
		return domain.ProcurementRequest{}, fmt.Errorf("governance evaluate: %w", err)
	}
	evalBytes, err := json.Marshal(eval)
	if err != nil {
		return domain.ProcurementRequest{}, err
	}
	req.EvaluationJSON = evalBytes
	req.UpdatedAt = time.Now()

	switch eval.Decision {
	case kerneldomain.DecisionDeny:
		req.Status = domain.StatusRejected
	case kerneldomain.DecisionRequireApproval:
		req.Status = domain.StatusPendingApproval
	case kerneldomain.DecisionAllow:
		req.Status = domain.StatusApproved
	default:
		req.Status = domain.StatusPendingApproval
	}

	out, err := u.repo.Update(ctx, req)
	if err != nil {
		return domain.ProcurementRequest{}, err
	}

	if eval.Decision == kerneldomain.DecisionAllow && u.purchases != nil {
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

	u.logAudit(ctx, orgID, actor, "procurement_request.submitted", out.ID.String(), map[string]any{"decision": eval.Decision})
	if u.timeline != nil {
		_ = u.timeline.RecordEvent(ctx, orgID, "procurement_request", out.ID, "procurement_request.submitted",
			"Solicitud de compra enviada", out.Title, actor, map[string]any{"decision": string(eval.Decision)})
	}
	u.emitWebhook(ctx, orgID, "procurement_request.submitted", map[string]any{
		"procurement_request_id": out.ID.String(),
		"decision":             string(eval.Decision),
		"status":                 string(out.Status),
		"purchase_id":          nullableUUIDPtr(out.PurchaseID),
	})
	return out, nil
}

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
	req.Status = domain.StatusApproved
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
		"purchase_id":          nullableUUIDPtr(out.PurchaseID),
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
	req.Status = domain.StatusRejected
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
