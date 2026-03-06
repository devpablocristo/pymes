package appointments

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/appointments/handler/dto"
	appointmentsdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/appointments/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/backend/internal/shared/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, orgID uuid.UUID, from, to *time.Time, status, assigned string, limit int) ([]appointmentsdomain.Appointment, error)
	Create(ctx context.Context, in appointmentsdomain.Appointment) (appointmentsdomain.Appointment, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (appointmentsdomain.Appointment, error)
	Update(ctx context.Context, in appointmentsdomain.Appointment, actor string) (appointmentsdomain.Appointment, error)
	Cancel(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/appointments", rbac.RequirePermission("appointments", "read"), h.List)
	auth.POST("/appointments", rbac.RequirePermission("appointments", "create"), h.Create)
	auth.GET("/appointments/:id", rbac.RequirePermission("appointments", "read"), h.Get)
	auth.PUT("/appointments/:id", rbac.RequirePermission("appointments", "update"), h.Update)
	auth.DELETE("/appointments/:id", rbac.RequirePermission("appointments", "delete"), h.Delete)
}

func (h *Handler) List(c *gin.Context) {
	orgID, ok := authOrg(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	from, err := parseDateQuery(c.Query("from"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from"})
		return
	}
	to, err := parseDateQuery(c.Query("to"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to"})
		return
	}
	items, err := h.uc.List(c.Request.Context(), orgID, from, to, c.Query("status"), c.Query("assigned_to"), limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) Create(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	var req dto.CreateAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	payload, err := createPayload(orgID, req, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	out, err := h.uc.Create(c.Request.Context(), payload)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) Get(c *gin.Context) {
	orgID, id, ok := authOrgAndID(c)
	if !ok {
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Update(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req dto.UpdateAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	payload, err := updatePayload(orgID, id, req)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	out, err := h.uc.Update(c.Request.Context(), payload, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Delete(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, id, ok := authOrgAndID(c)
	if !ok {
		return
	}
	if err := h.uc.Cancel(c.Request.Context(), orgID, id, authCtx.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func createPayload(orgID uuid.UUID, req dto.CreateAppointmentRequest, actor string) (appointmentsdomain.Appointment, error) {
	startAt, err := time.Parse(time.RFC3339, strings.TrimSpace(req.StartAt))
	if err != nil {
		return appointmentsdomain.Appointment{}, httperrors.ErrBadInput
	}
	endAt := time.Time{}
	if strings.TrimSpace(req.EndAt) != "" {
		endAt, err = time.Parse(time.RFC3339, strings.TrimSpace(req.EndAt))
		if err != nil {
			return appointmentsdomain.Appointment{}, httperrors.ErrBadInput
		}
	}
	var customerID *uuid.UUID
	if req.CustomerID != nil && strings.TrimSpace(*req.CustomerID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.CustomerID))
		if err != nil {
			return appointmentsdomain.Appointment{}, httperrors.ErrBadInput
		}
		customerID = &id
	}
	return appointmentsdomain.Appointment{
		OrgID:         orgID,
		CustomerID:    customerID,
		CustomerName:  strings.TrimSpace(req.CustomerName),
		CustomerPhone: strings.TrimSpace(req.CustomerPhone),
		Title:         strings.TrimSpace(req.Title),
		Description:   strings.TrimSpace(req.Description),
		Status:        req.Status,
		StartAt:       startAt.UTC(),
		EndAt:         endAt.UTC(),
		Duration:      req.Duration,
		Location:      strings.TrimSpace(req.Location),
		AssignedTo:    strings.TrimSpace(req.AssignedTo),
		Color:         strings.TrimSpace(req.Color),
		Notes:         strings.TrimSpace(req.Notes),
		Metadata:      req.Metadata,
		CreatedBy:     actor,
	}, nil
}

func updatePayload(orgID, id uuid.UUID, req dto.UpdateAppointmentRequest) (appointmentsdomain.Appointment, error) {
	payload := appointmentsdomain.Appointment{OrgID: orgID, ID: id, Metadata: req.Metadata}
	if req.CustomerID != nil && strings.TrimSpace(*req.CustomerID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*req.CustomerID))
		if err != nil {
			return appointmentsdomain.Appointment{}, httperrors.ErrBadInput
		}
		payload.CustomerID = &parsed
	}
	if req.CustomerName != nil {
		payload.CustomerName = strings.TrimSpace(*req.CustomerName)
	}
	if req.CustomerPhone != nil {
		payload.CustomerPhone = strings.TrimSpace(*req.CustomerPhone)
	}
	if req.Title != nil {
		payload.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		payload.Description = strings.TrimSpace(*req.Description)
	}
	if req.Status != nil {
		payload.Status = strings.TrimSpace(*req.Status)
	}
	if req.StartAt != nil && strings.TrimSpace(*req.StartAt) != "" {
		startAt, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.StartAt))
		if err != nil {
			return appointmentsdomain.Appointment{}, httperrors.ErrBadInput
		}
		payload.StartAt = startAt.UTC()
	}
	if req.EndAt != nil && strings.TrimSpace(*req.EndAt) != "" {
		endAt, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.EndAt))
		if err != nil {
			return appointmentsdomain.Appointment{}, httperrors.ErrBadInput
		}
		payload.EndAt = endAt.UTC()
	}
	if req.Duration != nil {
		payload.Duration = *req.Duration
	}
	if req.Location != nil {
		payload.Location = strings.TrimSpace(*req.Location)
	}
	if req.AssignedTo != nil {
		payload.AssignedTo = strings.TrimSpace(*req.AssignedTo)
	}
	if req.Color != nil {
		payload.Color = strings.TrimSpace(*req.Color)
	}
	if req.Notes != nil {
		payload.Notes = strings.TrimSpace(*req.Notes)
	}
	return payload, nil
}

func parseDateQuery(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func authOrg(c *gin.Context) (uuid.UUID, bool) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, false
	}
	return orgID, true
}

func authOrgAndID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	orgID, ok := authOrg(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, id, true
}
