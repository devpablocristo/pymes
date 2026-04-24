package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
)

type PermissionChecker interface {
	HasPermission(ctx context.Context, orgID, actor, role string, scopes []string, authMethod, resource, action string) bool
}

type Handler struct {
	uc      *Usecases
	checker PermissionChecker
}

func NewHandler(uc *Usecases, checker PermissionChecker) *Handler {
	return &Handler{uc: uc, checker: checker}
}

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup) {
	agent := auth.Group("/agent")
	agent.GET("/capabilities", h.ListCapabilities)
	agent.GET("/capabilities/:id", h.GetCapability)
	agent.POST("/confirmations", h.CreateConfirmation)
	agent.POST("/actions/:id/dry-run", h.DryRun)
	agent.POST("/actions/:id/execute", h.Execute)
	agent.GET("/events", h.ListEvents)
}

func (h *Handler) ListCapabilities(c *gin.Context) {
	auth := authContext(c)
	channel := Channel(strings.TrimSpace(c.Query("channel")))
	items := make([]Capability, 0)
	for _, cap := range h.uc.ListCapabilities() {
		if channel != "" && !channelAllowed(cap, channel) {
			continue
		}
		if !h.allowed(c.Request.Context(), auth, cap) {
			continue
		}
		items = append(items, cap)
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) GetCapability(c *gin.Context) {
	auth := authContext(c)
	cap, ok := h.uc.GetCapability(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": "capability_not_found", "message": "capability no encontrada"})
		return
	}
	if !h.allowed(c.Request.Context(), auth, cap) {
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "sin permiso para esta capability"})
		return
	}
	c.JSON(http.StatusOK, cap)
}

type createConfirmationRequest struct {
	CapabilityID string          `json:"capability_id"`
	Payload      json.RawMessage `json:"payload"`
	PayloadHash  string          `json:"payload_hash"`
	HumanSummary string          `json:"human_summary"`
	ExpiresAt    *time.Time      `json:"expires_at"`
}

func (h *Handler) CreateConfirmation(c *gin.Context) {
	auth := authContext(c)
	var req createConfirmationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "body invalido"})
		return
	}
	cap, ok := h.uc.GetCapability(req.CapabilityID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": "capability_not_found", "message": "capability no encontrada"})
		return
	}
	if !h.allowed(c.Request.Context(), auth, cap) {
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "sin permiso para esta capability"})
		return
	}
	out, err := h.uc.CreateConfirmation(c.Request.Context(), CreateConfirmationInput{
		Auth:         auth,
		CapabilityID: req.CapabilityID,
		Payload:      req.Payload,
		PayloadHash:  req.PayloadHash,
		HumanSummary: req.HumanSummary,
		ExpiresAt:    req.ExpiresAt,
	})
	if err != nil {
		respondAgentError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

type actionRequest struct {
	Payload         json.RawMessage `json:"payload"`
	Channel         string          `json:"channel"`
	ConfirmationID  string          `json:"confirmation_id"`
	ReviewRequestID string          `json:"review_request_id"`
	Reason          string          `json:"reason"`
}

func (h *Handler) DryRun(c *gin.Context) {
	auth := authContext(c)
	var req actionRequest
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "body invalido"})
		return
	}
	cap, ok := h.uc.GetCapability(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": "capability_not_found", "message": "capability no encontrada"})
		return
	}
	if !h.allowed(c.Request.Context(), auth, cap) {
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "sin permiso para esta capability"})
		return
	}
	out, err := h.uc.DryRun(c.Request.Context(), DryRunInput{
		Auth:         auth,
		CapabilityID: cap.ID,
		Payload:      req.Payload,
		Channel:      Channel(req.Channel),
		Reason:       req.Reason,
	})
	if err != nil {
		respondAgentError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Execute(c *gin.Context) {
	auth := authContext(c)
	cap, ok := h.uc.GetCapability(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": "capability_not_found", "message": "capability no encontrada"})
		return
	}
	if !h.allowed(c.Request.Context(), auth, cap) {
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "sin permiso para esta capability"})
		return
	}
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "body invalido"})
		return
	}
	if requiresExternalSignature(auth, cap) {
		if err := validateExternalSignature(c.Request, raw, time.Now().UTC()); err != nil {
			respondAgentError(c, err)
			return
		}
	}
	var req actionRequest
	if len(strings.TrimSpace(string(raw))) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "body invalido"})
			return
		}
	}
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	requestID := strings.TrimSpace(c.GetHeader("X-Pymes-Request-Id"))
	result, err := h.uc.Execute(c.Request.Context(), ExecuteInput{
		Auth:            auth,
		CapabilityID:    cap.ID,
		Payload:         req.Payload,
		Channel:         Channel(req.Channel),
		ConfirmationID:  req.ConfirmationID,
		ReviewRequestID: req.ReviewRequestID,
		Reason:          req.Reason,
		IdempotencyKey:  idempotencyKey,
		RequestID:       requestID,
	})
	if err != nil {
		respondAgentError(c, err)
		return
	}
	c.JSON(result.StatusCode, result.Output)
}

func (h *Handler) ListEvents(c *gin.Context) {
	auth := authContext(c)
	limit := 100
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	items, err := h.uc.ListEvents(c.Request.Context(), auth, limit, c.Query("capability_id"), c.Query("request_id"))
	if err != nil {
		respondAgentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) allowed(ctx context.Context, auth ActorContext, cap Capability) bool {
	if h.checker == nil {
		return false
	}
	return h.checker.HasPermission(ctx, auth.OrgID, auth.Actor, auth.Role, auth.Scopes, auth.AuthMethod, cap.RBACResource, cap.RBACAction)
}

func authContext(c *gin.Context) ActorContext {
	auth := handlers.GetAuthContext(c)
	return ActorContext{
		OrgID:      auth.OrgID,
		Actor:      auth.Actor,
		Role:       auth.Role,
		Scopes:     auth.Scopes,
		AuthMethod: auth.AuthMethod,
	}
}

func requiresExternalSignature(auth ActorContext, cap Capability) bool {
	if !strings.EqualFold(strings.TrimSpace(auth.AuthMethod), "api_key") {
		return false
	}
	return cap.RiskLevel != RiskRead
}

func respondAgentError(c *gin.Context, err error) {
	if status, code, message, ok := errorStatus(err); ok {
		c.JSON(status, gin.H{"code": code, "message": message})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
}
