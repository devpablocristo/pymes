package dto

type UpdateTenantSettingsRequest struct {
	PlanCode   string         `json:"plan_code"`
	HardLimits map[string]any `json:"hard_limits"`
}
