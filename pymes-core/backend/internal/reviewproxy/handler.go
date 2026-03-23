package reviewproxy

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/reviewproxy/handler/dto"
)

// Handler proxies requests del frontend a Nexus Review API.
type Handler struct {
	client *Client
}

// NewHandler crea un nuevo handler de review proxy.
func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

// RegisterRoutes registra las rutas del proxy en el router group autenticado.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	review := rg.Group("/review")
	{
		// Políticas
		review.GET("/policies", h.listPolicies)
		review.POST("/policies", h.createPolicy)
		review.PATCH("/policies/:id", h.updatePolicy)
		review.DELETE("/policies/:id", h.deletePolicy)

		// Action types
		review.GET("/action-types", h.listActionTypes)

		// Aprobaciones
		review.GET("/approvals/pending", h.listPendingApprovals)
		review.POST("/approvals/:id/approve", h.approve)
		review.POST("/approvals/:id/reject", h.reject)

		// Templates de condición
		review.GET("/condition-templates/:action_type", h.getConditionTemplates)
	}
}

func (h *Handler) listPolicies(c *gin.Context) {
	status, data, err := h.client.ListPolicies(c.Request.Context())
	if err != nil {
		log.Error().Err(err).Msg("review proxy: list policies failed")
		c.JSON(http.StatusBadGateway, gin.H{"code": "review_unavailable", "message": "No se pudo conectar con el servicio de reglas"})
		return
	}
	c.Data(status, "application/json", data)
}

func (h *Handler) createPolicy(c *gin.Context) {
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

	// Armar body para Review
	reviewBody := map[string]string{
		"name":        req.Name,
		"action_type": req.ActionType,
		"expression":  expression,
		"effect":      req.Effect,
		"mode":        req.Mode,
	}

	status, data, err := h.client.CreatePolicy(c.Request.Context(), reviewBody)
	if err != nil {
		log.Error().Err(err).Msg("review proxy: create policy failed")
		c.JSON(http.StatusBadGateway, gin.H{"code": "review_unavailable", "message": "No se pudo conectar con el servicio de reglas"})
		return
	}
	c.Data(status, "application/json", data)
}

func (h *Handler) updatePolicy(c *gin.Context) {
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

	status, data, err := h.client.UpdatePolicy(c.Request.Context(), id, updates)
	if err != nil {
		log.Error().Err(err).Msg("review proxy: update policy failed")
		c.JSON(http.StatusBadGateway, gin.H{"code": "review_unavailable", "message": "No se pudo conectar con el servicio de reglas"})
		return
	}
	c.Data(status, "application/json", data)
}

func (h *Handler) deletePolicy(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "ID de regla requerido"})
		return
	}

	status, err := h.client.DeletePolicy(c.Request.Context(), id)
	if err != nil {
		log.Error().Err(err).Msg("review proxy: delete policy failed")
		c.JSON(http.StatusBadGateway, gin.H{"code": "review_unavailable", "message": "No se pudo conectar con el servicio de reglas"})
		return
	}
	c.Status(status)
}

func (h *Handler) listActionTypes(c *gin.Context) {
	status, data, err := h.client.ListActionTypes(c.Request.Context())
	if err != nil {
		log.Error().Err(err).Msg("review proxy: list action types failed")
		c.JSON(http.StatusBadGateway, gin.H{"code": "review_unavailable", "message": "No se pudo conectar con el servicio de reglas"})
		return
	}
	c.Data(status, "application/json", data)
}

func (h *Handler) listPendingApprovals(c *gin.Context) {
	status, data, err := h.client.ListPendingApprovals(c.Request.Context())
	if err != nil {
		log.Error().Err(err).Msg("review proxy: list pending approvals failed")
		c.JSON(http.StatusBadGateway, gin.H{"code": "review_unavailable", "message": "No se pudo conectar con el servicio de reglas"})
		return
	}
	c.Data(status, "application/json", data)
}

func (h *Handler) approve(c *gin.Context) {
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

	status, data, err := h.client.Approve(c.Request.Context(), id, body)
	if err != nil {
		log.Error().Err(err).Msg("review proxy: approve failed")
		c.JSON(http.StatusBadGateway, gin.H{"code": "review_unavailable", "message": "No se pudo conectar con el servicio de reglas"})
		return
	}
	c.Data(status, "application/json", data)
}

func (h *Handler) reject(c *gin.Context) {
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

	status, data, err := h.client.Reject(c.Request.Context(), id, body)
	if err != nil {
		log.Error().Err(err).Msg("review proxy: reject failed")
		c.JSON(http.StatusBadGateway, gin.H{"code": "review_unavailable", "message": "No se pudo conectar con el servicio de reglas"})
		return
	}
	c.Data(status, "application/json", data)
}

func (h *Handler) getConditionTemplates(c *gin.Context) {
	actionType := strings.TrimSpace(c.Param("action_type"))
	templates := GetConditionTemplates(actionType)
	c.JSON(http.StatusOK, dto.ConditionTemplatesResponse{Templates: templates})
}

