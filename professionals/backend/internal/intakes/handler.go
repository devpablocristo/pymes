package intakes

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/professionals/backend/internal/intakes/handler/dto"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/intakes/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/professionals/backend/internal/shared/httperrors"
	"github.com/devpablocristo/pymes/pkgs/go-pkg/auth"
)

type usecasesPort interface {
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
	authGroup.GET("/intakes/:id", h.Get)
	authGroup.POST("/intakes", h.Create)
	authGroup.PUT("/intakes/:id", h.Update)
	authGroup.POST("/intakes/:id/submit", h.Submit)
}

func (h *Handler) Get(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
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
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	var req dto.CreateIntakeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
	if req.AppointmentID != nil && strings.TrimSpace(*req.AppointmentID) != "" {
		aid, err := uuid.Parse(*req.AppointmentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid appointment_id"})
			return
		}
		intake.AppointmentID = &aid
	}
	if req.CustomerPartyID != nil && strings.TrimSpace(*req.CustomerPartyID) != "" {
		cid, err := uuid.Parse(*req.CustomerPartyID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer_party_id"})
			return
		}
		intake.CustomerPartyID = &cid
	}
	if req.ProductID != nil && strings.TrimSpace(*req.ProductID) != "" {
		pid, err := uuid.Parse(*req.ProductID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id"})
			return
		}
		intake.ProductID = &pid
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
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req dto.UpdateIntakeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	input := UpdateInput{Payload: req.Payload}
	if req.AppointmentID != nil && strings.TrimSpace(*req.AppointmentID) != "" {
		aid, err := uuid.Parse(*req.AppointmentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid appointment_id"})
			return
		}
		input.AppointmentID = &aid
	}
	if req.CustomerPartyID != nil && strings.TrimSpace(*req.CustomerPartyID) != "" {
		cid, err := uuid.Parse(*req.CustomerPartyID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer_party_id"})
			return
		}
		input.CustomerPartyID = &cid
	}
	if req.ProductID != nil && strings.TrimSpace(*req.ProductID) != "" {
		pid, err := uuid.Parse(*req.ProductID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id"})
			return
		}
		input.ProductID = &pid
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
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
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
	if in.AppointmentID != nil {
		s := in.AppointmentID.String()
		item.AppointmentID = &s
	}
	if in.CustomerPartyID != nil {
		s := in.CustomerPartyID.String()
		item.CustomerPartyID = &s
	}
	if in.ProductID != nil {
		s := in.ProductID.String()
		item.ProductID = &s
	}
	return item
}
