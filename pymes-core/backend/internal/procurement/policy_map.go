package procurement

import (
	"strings"

	kerneldomain "github.com/devpablocristo/core/governance/go/kernel/usecases/domain"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/procurement/repository/models"
)

func mapDBPoliciesToKernel(rows []models.ProcurementPolicy) []kerneldomain.Policy {
	out := make([]kerneldomain.Policy, 0, len(rows))
	for _, row := range rows {
		if !row.Enabled {
			continue
		}
		effect := parseEffect(row.Effect)
		if effect == "" {
			continue
		}
		mode := kerneldomain.PolicyModeEnforce
		if strings.TrimSpace(row.Mode) == "shadow" {
			mode = kerneldomain.PolicyModeShadow
		}
		out = append(out, kerneldomain.Policy{
			ID:             row.ID.String(),
			Name:           row.Name,
			Expression:     row.Expression,
			Effect:         effect,
			Priority:       row.Priority,
			Mode:           mode,
			Enabled:        true,
			ActionFilter:   row.ActionFilter,
			SystemFilter:   row.SystemFilter,
		})
	}
	return out
}

func parseEffect(s string) kerneldomain.Decision {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "allow":
		return kerneldomain.DecisionAllow
	case "deny":
		return kerneldomain.DecisionDeny
	case "require_approval":
		return kerneldomain.DecisionRequireApproval
	default:
		return ""
	}
}

func defaultKernelPolicies() []kerneldomain.Policy {
	return []kerneldomain.Policy{
		{
			ID: "pymes-auto-low", Name: "Approve low spend",
			Expression: `double(request.params.estimated_total) < 1000`,
			Effect:     kerneldomain.DecisionAllow,
			Priority:   10,
			Mode:       kerneldomain.PolicyModeEnforce,
			Enabled:    true,
			ActionFilter: "procurement.submit",
			SystemFilter: "pymes",
		},
		{
			ID: "pymes-require-mid", Name: "Require approval medium",
			Expression: `double(request.params.estimated_total) < 50000`,
			Effect:     kerneldomain.DecisionRequireApproval,
			Priority:   20,
			Mode:       kerneldomain.PolicyModeEnforce,
			Enabled:    true,
			ActionFilter: "procurement.submit",
			SystemFilter: "pymes",
		},
		{
			ID: "pymes-deny-high", Name: "Deny very high",
			Expression: `double(request.params.estimated_total) >= 50000`,
			Effect:     kerneldomain.DecisionDeny,
			Priority:   30,
			Mode:       kerneldomain.PolicyModeEnforce,
			Enabled:    true,
			ActionFilter: "procurement.submit",
			SystemFilter: "pymes",
		},
	}
}
