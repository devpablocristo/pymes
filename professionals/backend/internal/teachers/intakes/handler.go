// Package intakes exposes HTTP handlers for professionals intake flows.
package intakes

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/intakes/handler/dto"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/intakes/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalgin"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/vertvalues"
)

type usecasesPort interface {
	List(ctx context.Context, orgID uuid.UUID) ([]domain.Intake, error)
	Create(ctx context.Context, in domain.Intake, actor string) (domain.Intake, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Intake, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.Intake, error)
	Submit(ctx context.Context, orgID, id uuid.UUID, actor string) (domain.Intake, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.GET("/intakes", h.List)
	authGroup.GET("/intakes/:id", h.Get)
	authGroup.POST("/intakes", h.Create)
	authGroup.PUT("/intakes/:id", h.Update)
	authGroup.POST("/intakes/:id/submit", h.Submit)
}

func (h *Handler) List(c *gin.Context) {
	orgID, ok := verticalgin.ParseAuthOrgID(c)
	if !ok {
		return
	}
	items, err := h.uc.List(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	out := make([]dto.IntakeItem, 0, len(items))
	for _, item := range items {
		out = append(out, toIntakeItem(item))
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (h *Handler) Get(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toIntakeItem(out))
}

func (h *Handler) Create(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, ok := verticalgin.ParseAuthOrgID(c)
	if !ok {
		return
	}
	var req dto.CreateIntakeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	profileID, err := uuid.Parse(req.ProfileID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile_id"})
		return
	}
	intake := domain.Intake{
		OrgID:     orgID,
		ProfileID: profileID,
		Status:    domain.IntakeStatusDraft,
		Payload:   req.Payload,
	}
	if req.BookingID != nil && strings.TrimSpace(*req.BookingID) != "" {
		intake.BookingID = vertvalues.ParseOptionalUUID(*req.BookingID)
		if intake.BookingID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid booking_id"})
			return
		}
	}
	if req.CustomerPartyID != nil && strings.TrimSpace(*req.CustomerPartyID) != "" {
		intake.CustomerPartyID = vertvalues.ParseOptionalUUID(*req.CustomerPartyID)
		if intake.CustomerPartyID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer_party_id"})
			return
		}
	}
	if req.ServiceID != nil && strings.TrimSpace(*req.ServiceID) != "" {
		intake.ServiceID = vertvalues.ParseOptionalUUID(*req.ServiceID)
		if intake.ServiceID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service_id"})
			return
		}
	}
	out, err := h.uc.Create(c.Request.Context(), intake, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toIntakeItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateIntakeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	input := UpdateInput{Payload: req.Payload}
	if req.BookingID != nil && strings.TrimSpace(*req.BookingID) != "" {
		input.BookingID = vertvalues.ParseOptionalUUID(*req.BookingID)
		if input.BookingID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid booking_id"})
			return
		}
	}
	if req.CustomerPartyID != nil && strings.TrimSpace(*req.CustomerPartyID) != "" {
		input.CustomerPartyID = vertvalues.ParseOptionalUUID(*req.CustomerPartyID)
		if input.CustomerPartyID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer_party_id"})
			return
		}
	}
	if req.ServiceID != nil && strings.TrimSpace(*req.ServiceID) != "" {
		input.ServiceID = vertvalues.ParseOptionalUUID(*req.ServiceID)
		if input.ServiceID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service_id"})
			return
		}
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, input, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toIntakeItem(out))
}

func (h *Handler) Submit(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	out, err := h.uc.Submit(c.Request.Context(), orgID, id, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toIntakeItem(out))
}

func toIntakeItem(in domain.Intake) dto.IntakeItem {
	item := dto.IntakeItem{
		ID:        in.ID.String(),
		OrgID:     in.OrgID.String(),
		ProfileID: in.ProfileID.String(),
		Status:    in.Status,
		Payload:   in.Payload,
		CreatedAt: in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: in.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if in.BookingID != nil {
		s := in.BookingID.String()
		item.BookingID = &s
	}
	if in.CustomerPartyID != nil {
		s := in.CustomerPartyID.String()
		item.CustomerPartyID = &s
	}
	if in.ServiceID != nil {
		s := in.ServiceID.String()
		item.ServiceID = &s
	}
	return item
}
