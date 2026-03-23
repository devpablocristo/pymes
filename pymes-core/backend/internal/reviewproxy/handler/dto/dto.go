// Package dto contiene los tipos de request/response para el proxy de Review.
package dto

// --- Policies ---

// CreatePolicyRequest es el request para crear una política de atención automática.
type CreatePolicyRequest struct {
	Name       string  `json:"name" binding:"required"`
	ActionType string  `json:"action_type" binding:"required"`
	Effect     string  `json:"effect" binding:"required,oneof=allow deny require_approval"`
	Condition  *string `json:"condition,omitempty"`
	Mode       string  `json:"mode" binding:"omitempty,oneof=enforced shadow"`
}

// UpdatePolicyRequest es el request para actualizar una política.
type UpdatePolicyRequest struct {
	Name      *string `json:"name,omitempty"`
	Effect    *string `json:"effect,omitempty" binding:"omitempty,oneof=allow deny require_approval"`
	Condition *string `json:"condition,omitempty"`
	Mode      *string `json:"mode,omitempty" binding:"omitempty,oneof=enforced shadow"`
}

// PolicyResponse es la representación de una política para el frontend.
type PolicyResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ActionType string `json:"action_type"`
	Effect     string `json:"effect"`
	Mode       string `json:"mode"`
	Expression string `json:"expression"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// PolicyListResponse es la lista de políticas.
type PolicyListResponse struct {
	Policies []PolicyResponse `json:"policies"`
	Total    int              `json:"total"`
}

// --- Action Types ---

// ActionTypeResponse es la representación de un action type para el frontend.
type ActionTypeResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	RiskClass   string `json:"risk_class"`
	Category    string `json:"category"`
}

// ActionTypeListResponse es la lista de action types.
type ActionTypeListResponse struct {
	ActionTypes []ActionTypeResponse `json:"action_types"`
}

// --- Approvals ---

// ApprovalResponse es la representación de una aprobación pendiente.
type ApprovalResponse struct {
	ID             string  `json:"id"`
	RequestID      string  `json:"request_id"`
	ActionType     string  `json:"action_type"`
	TargetResource string  `json:"target_resource"`
	Reason         string  `json:"reason"`
	RiskLevel      string  `json:"risk_level"`
	Status         string  `json:"status"`
	AISummary      *string `json:"ai_summary,omitempty"`
	CreatedAt      string  `json:"created_at"`
	ExpiresAt      *string `json:"expires_at,omitempty"`
}

// ApprovalListResponse es la lista de aprobaciones.
type ApprovalListResponse struct {
	Approvals []ApprovalResponse `json:"approvals"`
	Total     int                `json:"total"`
}

// ApprovalDecisionRequest es el request para aprobar/rechazar.
type ApprovalDecisionRequest struct {
	Note string `json:"note"`
}

// --- Condition Templates ---

// ConditionTemplate define un template de condición disponible para un action type.
type ConditionTemplate struct {
	Label      string `json:"label"`
	Pattern    string `json:"pattern"`
	ParamName  string `json:"param_name"`
	ParamType  string `json:"param_type"`
	DefaultVal string `json:"default_value"`
}

// ConditionTemplatesResponse es la respuesta con templates de condición.
type ConditionTemplatesResponse struct {
	Templates []ConditionTemplate `json:"templates"`
}
