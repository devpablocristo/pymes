// Package intakes exposes HTTP handlers for professionals intake flows.
package intakes

import (
	"context"
	"net/http"
	"strings"
	"time"

	crudpaths "github.com/devpablocristo/modules/crud/paths/go/paths"
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
	List(ctx context.Context, p ListParams) ([]domain.Intake, error)
	Create(ctx context.Context, in domain.Intake, actor string) (domain.Intake, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Intake, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.Intake, error)
	Submit(ctx context.Context, orgID, id uuid.UUID, actor string) (domain.Intake, error)
	Archive(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Delete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	const basePath = "/intakes"
	const itemPath = basePath + "/:id"

	authGroup.GET(basePath, h.List)
	authGroup.GET(basePath+"/"+crudpaths.SegmentArchived, h.ListArchived)
	authGroup.GET(itemPath, h.Get)
	authGroup.POST(basePath, h.Create)
	authGroup.PATCH(itemPath, h.Update)
	authGroup.DELETE(itemPath, h.Delete)
	authGroup.POST(itemPath+"/"+crudpaths.SegmentArchive, h.Archive)
	authGroup.POST(itemPath+"/"+crudpaths.SegmentRestore, h.Restore)
	authGroup.DELETE(itemPath+"/"+crudpaths.SegmentHard, h.HardDelete)
	authGroup.POST(itemPath+"/submit", h.Submit)
}

func (h *Handler) List(c *gin.Context) {
	h.list(c, false)
}

func (h *Handler) ListArchived(c *gin.Context) {
	h.list(c, true)
}

func (h *Handler) list(c *gin.Context, forceArchived bool) {
	orgID, ok := verticalgin.ParseAuthTenantID(c)
	if !ok {
		return
	}
	items, err := h.uc.List(c.Request.Context(), ListParams{
		OrgID: orgID,
		Archived: forceArchived || c.Query("archived") == "true",
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	out := make([]dto.IntakeItem, 0, len(items))
	for _, item := range items {
		out = append(out, toIntakeItem(item))
	}
	verticalgin.WriteListResponse(c, out, int64(len(out)), false, "")
}

func (h *Handler) Get(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
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

func (h *Handler) Delete(c *gin.Context) {
	h.Archive(c)
}

func (h *Handler) Archive(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.Archive(c.Request.Context(), orgID, id, auth.GetAuthContext(c).Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Restore(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.Restore(c.Request.Context(), orgID, id, auth.GetAuthContext(c).Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HardDelete(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.Delete(c.Request.Context(), orgID, id, auth.GetAuthContext(c).Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Create(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, ok := verticalgin.ParseAuthTenantID(c)
	if !ok {
		return
	}
	var req dto.CreateIntakeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		verticalgin.WriteValidation(c, "invalid request body")
		return
	}
	profileID, err := uuid.Parse(req.ProfileID)
	if err != nil {
		verticalgin.WriteValidation(c, "invalid profile_id")
		return
	}
	isFavorite := false
	if req.IsFavorite != nil {
		isFavorite = *req.IsFavorite
	}
	intake := domain.Intake{
		OrgID:   orgID,
		ProfileID:  profileID,
		Status:     domain.IntakeStatusDraft,
		IsFavorite: isFavorite,
		Tags:       req.Tags,
		Payload:    req.Payload,
	}
	if req.BookingID != nil && strings.TrimSpace(*req.BookingID) != "" {
		intake.BookingID = vertvalues.ParseOptionalUUID(*req.BookingID)
		if intake.BookingID == nil {
			verticalgin.WriteValidation(c, "invalid booking_id")
			return
		}
	}
	if req.CustomerPartyID != nil && strings.TrimSpace(*req.CustomerPartyID) != "" {
		intake.CustomerPartyID = vertvalues.ParseOptionalUUID(*req.CustomerPartyID)
		if intake.CustomerPartyID == nil {
			verticalgin.WriteValidation(c, "invalid customer_party_id")
			return
		}
	}
	if req.ServiceID != nil && strings.TrimSpace(*req.ServiceID) != "" {
		intake.ServiceID = vertvalues.ParseOptionalUUID(*req.ServiceID)
		if intake.ServiceID == nil {
			verticalgin.WriteValidation(c, "invalid service_id")
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
	orgID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateIntakeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		verticalgin.WriteValidation(c, "invalid request body")
		return
	}
	input := UpdateInput{
		Payload:    req.Payload,
		IsFavorite: req.IsFavorite,
		Tags:       req.Tags,
	}
	if req.BookingID != nil && strings.TrimSpace(*req.BookingID) != "" {
		input.BookingID = vertvalues.ParseOptionalUUID(*req.BookingID)
		if input.BookingID == nil {
			verticalgin.WriteValidation(c, "invalid booking_id")
			return
		}
	}
	if req.CustomerPartyID != nil && strings.TrimSpace(*req.CustomerPartyID) != "" {
		input.CustomerPartyID = vertvalues.ParseOptionalUUID(*req.CustomerPartyID)
		if input.CustomerPartyID == nil {
			verticalgin.WriteValidation(c, "invalid customer_party_id")
			return
		}
	}
	if req.ServiceID != nil && strings.TrimSpace(*req.ServiceID) != "" {
		input.ServiceID = vertvalues.ParseOptionalUUID(*req.ServiceID)
		if input.ServiceID == nil {
			verticalgin.WriteValidation(c, "invalid service_id")
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
	orgID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
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
	tags := in.Tags
	if tags == nil {
		tags = []string{}
	}
	item := dto.IntakeItem{
		ID:         in.ID.String(),
		OrgID:   in.OrgID.String(),
		ProfileID:  in.ProfileID.String(),
		Status:     in.Status,
		IsFavorite: in.IsFavorite,
		Tags:       tags,
		Payload:    in.Payload,
		CreatedAt:  in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  in.UpdatedAt.UTC().Format(time.RFC3339),
		DeletedAt:  formatOptionalTime(in.DeletedAt),
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

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
