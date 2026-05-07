package procurement

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/devpablocristo/core/governance/go/governanceclient"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/procurement/usecases/domain"
)

// PolicyCreateInput crea una política CEL por tenant.
type PolicyCreateInput struct {
	TenantID     uuid.UUID
	Actor        string
	Name         string
	Expression   string
	Effect       string
	Priority     int
	Mode         string
	Enabled      bool
	ActionFilter string
	SystemFilter string
}

// PolicyUpdateInput actualiza una política existente.
type PolicyUpdateInput struct {
	TenantID     uuid.UUID
	ID           uuid.UUID
	Actor        string
	Name         string
	Expression   string
	Effect       string
	Priority     int
	Mode         string
	Enabled      bool
	ActionFilter string
	SystemFilter string
}

// ListPoliciesForTenant lista las policies de procurement del tenant: proxy a
// Nexus, sin almacenamiento local en Pymes.
func (u *Usecases) ListPoliciesForTenant(ctx context.Context, tenantID uuid.UUID) ([]domain.ProcurementPolicy, error) {
	st, raw, err := u.governance.ListPoliciesForTenant(ctx, tenantID.String())
	if err != nil {
		return nil, fmt.Errorf("nexus list policies: %w", err)
	}
	if st >= 400 {
		return nil, fmt.Errorf("nexus list policies: status %d body %s", st, governanceclient.ParseErrorBody(raw))
	}
	var envelope struct {
		Data []governanceclient.PolicyResponse `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode list policies: %w", err)
	}
	out := make([]domain.ProcurementPolicy, 0, len(envelope.Data))
	for _, p := range envelope.Data {
		out = append(out, nexusPolicyToDomain(p, tenantID))
	}
	return out, nil
}

func (u *Usecases) GetPolicy(ctx context.Context, tenantID, id uuid.UUID) (domain.ProcurementPolicy, error) {
	st, raw, err := u.governance.GetPolicyForTenant(ctx, tenantID.String(), id.String())
	if err != nil {
		return domain.ProcurementPolicy{}, fmt.Errorf("nexus get policy: %w", err)
	}
	if st == http.StatusNotFound {
		return domain.ProcurementPolicy{}, domainerr.NotFoundf("procurement_policy", id.String())
	}
	if st >= 400 {
		return domain.ProcurementPolicy{}, fmt.Errorf("nexus get policy: status %d body %s", st, governanceclient.ParseErrorBody(raw))
	}
	var p governanceclient.PolicyResponse
	if err := json.Unmarshal(raw, &p); err != nil {
		return domain.ProcurementPolicy{}, fmt.Errorf("decode policy: %w", err)
	}
	return nexusPolicyToDomain(p, tenantID), nil
}

func (u *Usecases) CreatePolicy(ctx context.Context, in PolicyCreateInput) (domain.ProcurementPolicy, error) {
	if in.TenantID == uuid.Nil {
		return domain.ProcurementPolicy{}, domainerr.Validation("tenant_id is required")
	}
	actor := strings.TrimSpace(in.Actor)
	if actor == "" {
		return domain.ProcurementPolicy{}, domainerr.Validation("actor is required")
	}
	mode := normalizePolicyMode(in.Mode)
	if err := validatePolicyFields(in.Name, in.Expression, in.Effect, mode); err != nil {
		return domain.ProcurementPolicy{}, err
	}
	body := nexusPolicyCreateBody(in, mode)

	st, raw, err := u.governance.CreatePolicyForTenant(ctx, in.TenantID.String(), body)
	if err != nil {
		return domain.ProcurementPolicy{}, fmt.Errorf("nexus create policy: %w", err)
	}
	if st >= 400 {
		return domain.ProcurementPolicy{}, fmt.Errorf("nexus create policy: status %d body %s", st, governanceclient.ParseErrorBody(raw))
	}
	var p governanceclient.PolicyResponse
	if err := json.Unmarshal(raw, &p); err != nil {
		return domain.ProcurementPolicy{}, fmt.Errorf("decode created policy: %w", err)
	}
	out := nexusPolicyToDomain(p, in.TenantID)
	u.logPolicyAudit(ctx, in.TenantID, actor, "procurement_policy.created", out.ID.String(), map[string]any{"name": out.Name})
	u.emitWebhook(ctx, in.TenantID, "procurement_policy.created", map[string]any{
		"procurement_policy_id": out.ID.String(),
		"name":                  out.Name,
	})
	return out, nil
}

func (u *Usecases) UpdatePolicy(ctx context.Context, in PolicyUpdateInput) (domain.ProcurementPolicy, error) {
	actor := strings.TrimSpace(in.Actor)
	if actor == "" {
		return domain.ProcurementPolicy{}, domainerr.Validation("actor is required")
	}
	mode := normalizePolicyMode(in.Mode)
	if err := validatePolicyFields(in.Name, in.Expression, in.Effect, mode); err != nil {
		return domain.ProcurementPolicy{}, err
	}
	body := nexusPolicyUpdateBody(in, mode)

	st, raw, err := u.governance.UpdatePolicyForTenant(ctx, in.TenantID.String(), in.ID.String(), body)
	if err != nil {
		return domain.ProcurementPolicy{}, fmt.Errorf("nexus update policy: %w", err)
	}
	if st == http.StatusNotFound {
		return domain.ProcurementPolicy{}, domainerr.NotFoundf("procurement_policy", in.ID.String())
	}
	if st >= 400 {
		return domain.ProcurementPolicy{}, fmt.Errorf("nexus update policy: status %d body %s", st, governanceclient.ParseErrorBody(raw))
	}
	var p governanceclient.PolicyResponse
	if err := json.Unmarshal(raw, &p); err != nil {
		return domain.ProcurementPolicy{}, fmt.Errorf("decode updated policy: %w", err)
	}
	out := nexusPolicyToDomain(p, in.TenantID)
	u.logPolicyAudit(ctx, in.TenantID, actor, "procurement_policy.updated", out.ID.String(), map[string]any{"name": out.Name})
	u.emitWebhook(ctx, in.TenantID, "procurement_policy.updated", map[string]any{
		"procurement_policy_id": out.ID.String(),
		"name":                  out.Name,
	})
	return out, nil
}

func (u *Usecases) DeletePolicy(ctx context.Context, tenantID, id uuid.UUID, actor string) error {
	if strings.TrimSpace(actor) == "" {
		return domainerr.Validation("actor is required")
	}
	st, err := u.governance.DeletePolicyForTenant(ctx, tenantID.String(), id.String())
	if err != nil {
		return fmt.Errorf("nexus delete policy: %w", err)
	}
	if st == http.StatusNotFound {
		return domainerr.NotFoundf("procurement_policy", id.String())
	}
	if st >= 400 {
		return fmt.Errorf("nexus delete policy: status %d", st)
	}
	u.logPolicyAudit(ctx, tenantID, actor, "procurement_policy.deleted", id.String(), nil)
	u.emitWebhook(ctx, tenantID, "procurement_policy.deleted", map[string]any{"procurement_policy_id": id.String()})
	return nil
}

func validatePolicyFields(name, expression, effect, mode string) error {
	if strings.TrimSpace(name) == "" {
		return domainerr.Validation("name is required")
	}
	if strings.TrimSpace(expression) == "" {
		return domainerr.Validation("expression is required")
	}
	switch strings.TrimSpace(effect) {
	case governanceclient.PolicyEffectAllow,
		governanceclient.PolicyEffectDeny,
		governanceclient.PolicyEffectRequireApproval:
	default:
		return domainerr.Validation("invalid effect")
	}
	switch mode {
	case governanceclient.PolicyModeEnforced, governanceclient.PolicyModeShadow:
	default:
		return domainerr.Validation("invalid mode")
	}
	return nil
}

// normalizePolicyMode acepta los strings que la UI Pymes envía hoy
// ("enforce", "shadow", vacío) y los traduce al wire format canónico de
// Nexus ("enforced" / "shadow"). Compat hacia atrás del UI legacy.
func normalizePolicyMode(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "", "enforce", "enforced":
		return governanceclient.PolicyModeEnforced
	case "shadow":
		return governanceclient.PolicyModeShadow
	default:
		return strings.TrimSpace(raw)
	}
}

func (u *Usecases) logPolicyAudit(ctx context.Context, tenantID uuid.UUID, actor, action, resourceID string, payload map[string]any) {
	if u.audit == nil {
		return
	}
	u.audit.Log(ctx, tenantID.String(), actor, action, "procurement_policy", resourceID, payload)
}

// nexusPolicyToDomain mapea el shape público de Nexus a la representación
// que la UI de Pymes consume. CreatedAt/UpdatedAt vienen como RFC3339.
func nexusPolicyToDomain(p governanceclient.PolicyResponse, tenantID uuid.UUID) domain.ProcurementPolicy {
	id, _ := uuid.Parse(p.ID)
	created, _ := time.Parse(time.RFC3339, p.CreatedAt)
	updated, _ := time.Parse(time.RFC3339, p.UpdatedAt)
	actionFilter := ""
	if p.ActionType != nil {
		actionFilter = *p.ActionType
	}
	systemFilter := ""
	if p.TargetSystem != nil {
		systemFilter = *p.TargetSystem
	}
	return domain.ProcurementPolicy{
		ID:           id,
		TenantID:     tenantID,
		Name:         p.Name,
		Expression:   p.Expression,
		Effect:       p.Effect,
		Priority:     p.Priority,
		Mode:         p.Mode,
		Enabled:      p.Enabled,
		ActionFilter: actionFilter,
		SystemFilter: systemFilter,
		CreatedAt:    created,
		UpdatedAt:    updated,
	}
}

func nexusPolicyCreateBody(in PolicyCreateInput, mode string) governanceclient.CreatePolicyRequest {
	actionType := defaultString(strings.TrimSpace(in.ActionFilter), "procurement.submit")
	system := defaultString(strings.TrimSpace(in.SystemFilter), "pymes")
	return governanceclient.CreatePolicyRequest{
		Name:         strings.TrimSpace(in.Name),
		Description:  "",
		ActionType:   &actionType,
		TargetSystem: &system,
		Expression:   strings.TrimSpace(in.Expression),
		Effect:       strings.TrimSpace(in.Effect),
		Priority:     in.Priority,
		Mode:         mode,
		Enabled:      in.Enabled,
	}
}

func nexusPolicyUpdateBody(in PolicyUpdateInput, mode string) governanceclient.UpdatePolicyRequest {
	name := strings.TrimSpace(in.Name)
	expr := strings.TrimSpace(in.Expression)
	effect := strings.TrimSpace(in.Effect)
	actionType := defaultString(strings.TrimSpace(in.ActionFilter), "procurement.submit")
	system := defaultString(strings.TrimSpace(in.SystemFilter), "pymes")
	return governanceclient.UpdatePolicyRequest{
		Name:         &name,
		Expression:   &expr,
		Effect:       &effect,
		Priority:     &in.Priority,
		Mode:         &mode,
		Enabled:      &in.Enabled,
		ActionType:   &actionType,
		TargetSystem: &system,
	}
}
