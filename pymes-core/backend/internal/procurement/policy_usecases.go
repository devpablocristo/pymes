package procurement

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	kerneldomain "github.com/devpablocristo/core/governance/go/kernel/usecases/domain"
	"github.com/devpablocristo/core/backend/go/apperror"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/procurement/repository/models"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/procurement/usecases/domain"
)

// PolicyCreateInput crea una política CEL por organización.
type PolicyCreateInput struct {
	OrgID          uuid.UUID
	Actor          string
	Name           string
	Expression     string
	Effect         string
	Priority       int
	Mode           string
	Enabled        bool
	ActionFilter   string
	SystemFilter   string
}

// PolicyUpdateInput actualiza una política existente.
type PolicyUpdateInput struct {
	OrgID          uuid.UUID
	ID             uuid.UUID
	Actor          string
	Name           string
	Expression     string
	Effect         string
	Priority       int
	Mode           string
	Enabled        bool
	ActionFilter   string
	SystemFilter   string
}

func (u *Usecases) ListPoliciesForOrg(ctx context.Context, orgID uuid.UUID) ([]domain.ProcurementPolicy, error) {
	rows, err := u.repo.ListPolicies(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ProcurementPolicy, 0, len(rows))
	for _, m := range rows {
		out = append(out, procurementPolicyModelToDomain(m))
	}
	return out, nil
}

func (u *Usecases) GetPolicy(ctx context.Context, orgID, id uuid.UUID) (domain.ProcurementPolicy, error) {
	p, err := u.repo.GetPolicyByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.ProcurementPolicy{}, apperror.NewNotFound("procurement_policy", id.String())
		}
		return domain.ProcurementPolicy{}, err
	}
	return p, nil
}

func (u *Usecases) CreatePolicy(ctx context.Context, in PolicyCreateInput) (domain.ProcurementPolicy, error) {
	if in.OrgID == uuid.Nil {
		return domain.ProcurementPolicy{}, apperror.NewBadInput("org_id is required")
	}
	actor := strings.TrimSpace(in.Actor)
	if actor == "" {
		return domain.ProcurementPolicy{}, apperror.NewBadInput("actor is required")
	}
	mode := strings.TrimSpace(in.Mode)
	if mode == "" {
		mode = string(kerneldomain.PolicyModeEnforce)
	}
	if err := validatePolicyFields(in.Name, in.Expression, in.Effect, mode); err != nil {
		return domain.ProcurementPolicy{}, err
	}
	now := time.Now()
	p := domain.ProcurementPolicy{
		ID:           uuid.New(),
		OrgID:        in.OrgID,
		Name:         strings.TrimSpace(in.Name),
		Expression:   strings.TrimSpace(in.Expression),
		Effect:       strings.TrimSpace(in.Effect),
		Priority:     in.Priority,
		Mode:         mode,
		Enabled:      in.Enabled,
		ActionFilter: defaultString(strings.TrimSpace(in.ActionFilter), "procurement.submit"),
		SystemFilter: defaultString(strings.TrimSpace(in.SystemFilter), "pymes"),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	out, err := u.repo.SavePolicy(ctx, p)
	if err != nil {
		return domain.ProcurementPolicy{}, err
	}
	u.logPolicyAudit(ctx, in.OrgID, actor, "procurement_policy.created", out.ID.String(), map[string]any{"name": out.Name})
	u.emitWebhook(ctx, in.OrgID, "procurement_policy.created", map[string]any{
		"procurement_policy_id": out.ID.String(),
		"name":                  out.Name,
	})
	return out, nil
}

func (u *Usecases) UpdatePolicy(ctx context.Context, in PolicyUpdateInput) (domain.ProcurementPolicy, error) {
	cur, err := u.repo.GetPolicyByID(ctx, in.OrgID, in.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.ProcurementPolicy{}, apperror.NewNotFound("procurement_policy", in.ID.String())
		}
		return domain.ProcurementPolicy{}, err
	}
	actor := strings.TrimSpace(in.Actor)
	if actor == "" {
		return domain.ProcurementPolicy{}, apperror.NewBadInput("actor is required")
	}
	mode := strings.TrimSpace(in.Mode)
	if mode == "" {
		mode = string(kerneldomain.PolicyModeEnforce)
	}
	if err := validatePolicyFields(in.Name, in.Expression, in.Effect, mode); err != nil {
		return domain.ProcurementPolicy{}, err
	}
	cur.Name = strings.TrimSpace(in.Name)
	cur.Expression = strings.TrimSpace(in.Expression)
	cur.Effect = strings.TrimSpace(in.Effect)
	cur.Priority = in.Priority
	cur.Mode = mode
	cur.Enabled = in.Enabled
	cur.ActionFilter = defaultString(strings.TrimSpace(in.ActionFilter), "procurement.submit")
	cur.SystemFilter = defaultString(strings.TrimSpace(in.SystemFilter), "pymes")
	cur.UpdatedAt = time.Now()
	out, err := u.repo.SavePolicy(ctx, cur)
	if err != nil {
		return domain.ProcurementPolicy{}, err
	}
	u.logPolicyAudit(ctx, in.OrgID, actor, "procurement_policy.updated", out.ID.String(), map[string]any{"name": out.Name})
	u.emitWebhook(ctx, in.OrgID, "procurement_policy.updated", map[string]any{
		"procurement_policy_id": out.ID.String(),
		"name":                  out.Name,
	})
	return out, nil
}

func (u *Usecases) DeletePolicy(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if strings.TrimSpace(actor) == "" {
		return apperror.NewBadInput("actor is required")
	}
	if err := u.repo.DeletePolicy(ctx, orgID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return apperror.NewNotFound("procurement_policy", id.String())
		}
		return err
	}
	u.logPolicyAudit(ctx, orgID, actor, "procurement_policy.deleted", id.String(), nil)
	u.emitWebhook(ctx, orgID, "procurement_policy.deleted", map[string]any{"procurement_policy_id": id.String()})
	return nil
}

func validatePolicyFields(name, expression, effect, mode string) error {
	if strings.TrimSpace(name) == "" {
		return apperror.NewBadInput("name is required")
	}
	if strings.TrimSpace(expression) == "" {
		return apperror.NewBadInput("expression is required")
	}
	e := strings.TrimSpace(effect)
	switch kerneldomain.Decision(e) {
	case kerneldomain.DecisionAllow, kerneldomain.DecisionDeny, kerneldomain.DecisionRequireApproval:
	default:
		return apperror.NewBadInput("invalid effect")
	}
	m := strings.TrimSpace(mode)
	switch kerneldomain.PolicyMode(m) {
	case kerneldomain.PolicyModeEnforce, kerneldomain.PolicyModeShadow:
	default:
		return apperror.NewBadInput("invalid mode")
	}
	return nil
}

func procurementPolicyModelToDomain(m models.ProcurementPolicy) domain.ProcurementPolicy {
	return domain.ProcurementPolicy{
		ID:           m.ID,
		OrgID:        m.OrgID,
		Name:         m.Name,
		Expression:   m.Expression,
		Effect:       m.Effect,
		Priority:     m.Priority,
		Mode:         m.Mode,
		Enabled:      m.Enabled,
		ActionFilter: m.ActionFilter,
		SystemFilter: m.SystemFilter,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

func (u *Usecases) logPolicyAudit(ctx context.Context, orgID uuid.UUID, actor, action, resourceID string, payload map[string]any) {
	if u.audit == nil {
		return
	}
	u.audit.Log(ctx, orgID.String(), actor, action, "procurement_policy", resourceID, payload)
}
