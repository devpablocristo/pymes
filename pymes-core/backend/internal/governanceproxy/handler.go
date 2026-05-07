package governanceproxy

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/governanceproxy/handler/dto"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
)

type governanceClient interface {
	ListPoliciesForTenant(ctx context.Context, tenantID string) (int, []byte, error)
	CreatePolicyForTenant(ctx context.Context, tenantID string, body any) (int, []byte, error)
	UpdatePolicyForTenant(ctx context.Context, tenantID, id string, updates any) (int, []byte, error)
	DeletePolicyForTenant(ctx context.Context, tenantID, id string) (int, error)
	ListActionTypes(ctx context.Context) (int, []byte, error)
	ListPendingApprovalsForTenant(ctx context.Context, tenantID string) (int, []byte, error)
	ApproveForTenant(ctx context.Context, tenantID, id string, body any) (int, []byte, error)
	RejectForTenant(ctx context.Context, tenantID, id string, body any) (int, []byte, error)
}

// Handler proxies requests del frontend a Nexus Governance API.
type Handler struct {
	client governanceClient
}

// NewHandler crea un nuevo handler de governance proxy.
func NewHandler(client governanceClient) *Handler {
	return &Handler{client: client}
}

// RegisterRoutes registra las rutas del proxy en el router group autenticado.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	governance := rg.Group("/governance")
	{
		// Políticas
		governance.GET("/policies", h.listPolicies)
		governance.POST("/policies", h.createPolicy)
		governance.PATCH("/policies/:id", h.updatePolicy)
		governance.DELETE("/policies/:id", h.deletePolicy)

		// Action types
		governance.GET("/action-types", h.listActionTypes)

		// Aprobaciones
		governance.GET("/approvals/pending", h.listPendingApprovals)
		governance.POST("/approvals/:id/approve", h.approve)
		governance.POST("/approvals/:id/reject", h.reject)

		// Templates de condición
		governance.GET("/condition-templates/:action_type", h.getConditionTemplates)
	}
}

func (h *Handler) listPolicies(c *gin.Context) {
	tenantID, ok := tenantIDFromAuth(c)
	if !ok {
		return
	}
	status, data, err := h.client.ListPoliciesForTenant(c.Request.Context(), tenantID)
	if err != nil {
		log.Error().Err(err).Msg("governance proxy: list policies failed")
		c.JSON(http.StatusBadGateway, gin.H{"code": "governance_unavailable", "message": "No se pudo conectar con el servicio de reglas"})
		return
	}
	c.Data(status, "application/json", data)
}

func (h *Handler) createPolicy(c *gin.Context) {
	tenantID, ok := tenantIDFromAuth(c)
	if !ok {
		return
	}
	var req dto.CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "Datos de la regla incompletos"})
		return
	}

	if strings.TrimSpace(req.Mode) == "" {
		req.Mode = "enforced"
	}

	// Construir expresión CEL desde la condición
	expression := BuildCELExpression(req.ActionType, req.Condition)

	// Armar body para Governance
	governanceBody := map[string]string{
		"name":        req.Name,
		"action_type": req.ActionType,
		"expression":  expression,
		"effect":      req.Effect,
		"mode":        req.Mode,
	}

	status, data, err := h.client.CreatePolicyForTenant(c.Request.Context(), tenantID, governanceBody)
	if err != nil {
		log.Error().Err(err).Msg("governance proxy: create policy failed")
		c.JSON(http.StatusBadGateway, gin.H{"code": "governance_unavailable", "message": "No se pudo conectar con el servicio de reglas"})
		return
	}
	c.Data(status, "application/json", data)
}

func (h *Handler) updatePolicy(c *gin.Context) {
	tenantID, ok := tenantIDFromAuth(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "ID de regla requerido"})
		return
	}

	var req dto.UpdatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "Datos de actualización inválidos"})
		return
	}

	// Si hay condición nueva, reconstruir expresión CEL
	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Effect != nil {
		updates["effect"] = *req.Effect
	}
	if req.Mode != nil {
		updates["mode"] = *req.Mode
	}
	if req.Condition != nil {
		updates["expression"] = BuildCELExpression("", req.Condition)
	}

	status, data, err := h.client.UpdatePolicyForTenant(c.Request.Context(), tenantID, id, updates)
	if err != nil {
		log.Error().Err(err).Msg("governance proxy: update policy failed")
		c.JSON(http.StatusBadGateway, gin.H{"code": "governance_unavailable", "message": "No se pudo conectar con el servicio de reglas"})
		return
	}
	c.Data(status, "application/json", data)
}

func (h *Handler) deletePolicy(c *gin.Context) {
	tenantID, ok := tenantIDFromAuth(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "ID de regla requerido"})
		return
	}

	status, err := h.client.DeletePolicyForTenant(c.Request.Context(), tenantID, id)
	if err != nil {
		log.Error().Err(err).Msg("governance proxy: delete policy failed")
		c.JSON(http.StatusBadGateway, gin.H{"code": "governance_unavailable", "message": "No se pudo conectar con el servicio de reglas"})
		return
	}
	c.Status(status)
}

func (h *Handler) listActionTypes(c *gin.Context) {
	status, data, err := h.client.ListActionTypes(c.Request.Context())
	if err != nil {
		log.Error().Err(err).Msg("governance proxy: list action types failed")
		c.JSON(http.StatusBadGateway, gin.H{"code": "governance_unavailable", "message": "No se pudo conectar con el servicio de reglas"})
		return
	}
	c.Data(status, "application/json", data)
}

func (h *Handler) listPendingApprovals(c *gin.Context) {
	tenantID, ok := tenantIDFromAuth(c)
	if !ok {
		return
	}
	status, data, err := h.client.ListPendingApprovalsForTenant(c.Request.Context(), tenantID)
	if err != nil {
		// La bandeja de notificaciones usa approvals como señal opcional.
		// Si Governance no está disponible en local o en entornos sin governance,
		// devolvemos lista vacía para no degradar toda la pantalla.
		log.Warn().Err(err).Msg("governance proxy: list pending approvals unavailable, returning empty list")
		c.JSON(http.StatusOK, dto.ApprovalListResponse{Approvals: []dto.ApprovalResponse{}, Total: 0})
		return
	}
	c.Data(status, "application/json", data)
}

func (h *Handler) approve(c *gin.Context) {
	tenantID, ok := tenantIDFromAuth(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "ID de aprobación requerido"})
		return
	}

	var req dto.ApprovalDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = dto.ApprovalDecisionRequest{}
	}

	body := map[string]string{"decided_by": "owner", "note": req.Note}

	status, data, err := h.client.ApproveForTenant(c.Request.Context(), tenantID, id, body)
	if err != nil {
		log.Error().Err(err).Msg("governance proxy: approve failed")
		c.JSON(http.StatusBadGateway, gin.H{"code": "governance_unavailable", "message": "No se pudo conectar con el servicio de reglas"})
		return
	}
	c.Data(status, "application/json", data)
}

func (h *Handler) reject(c *gin.Context) {
	tenantID, ok := tenantIDFromAuth(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "ID de aprobación requerido"})
		return
	}

	var req dto.ApprovalDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = dto.ApprovalDecisionRequest{}
	}

	body := map[string]string{"decided_by": "owner", "note": req.Note}

	status, data, err := h.client.RejectForTenant(c.Request.Context(), tenantID, id, body)
	if err != nil {
		log.Error().Err(err).Msg("governance proxy: reject failed")
		c.JSON(http.StatusBadGateway, gin.H{"code": "governance_unavailable", "message": "No se pudo conectar con el servicio de reglas"})
		return
	}
	c.Data(status, "application/json", data)
}

func (h *Handler) getConditionTemplates(c *gin.Context) {
	actionType := strings.TrimSpace(c.Param("action_type"))
	templates := GetConditionTemplates(actionType)
	c.JSON(http.StatusOK, dto.ConditionTemplatesResponse{Templates: templates})
}

func tenantIDFromAuth(c *gin.Context) (string, bool) {
	auth := handlers.GetAuthContext(c)
	tenantID := strings.TrimSpace(auth.TenantID)
	if tenantID == "" {
		c.JSON(http.StatusForbidden, gin.H{"code": "tenant_required", "message": "tenant activo requerido"})
		return "", false
	}
	return tenantID, true
}
