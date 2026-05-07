package customer_messaging

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/handler/dto"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

func (h *Handler) ListOptIns(c *gin.Context) {
	tenantID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	optIns, err := h.uc.ListOptIns(c.Request.Context(), tenantID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	items := make([]dto.OptInResponse, 0, len(optIns))
	for _, o := range optIns {
		items = append(items, toOptInResponse(o))
	}
	c.JSON(http.StatusOK, dto.OptInListResponse{OptIns: items, Total: len(items)})
}

func (h *Handler) RegisterOptIn(c *gin.Context) {
	tenantID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	var body dto.OptInRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		writeBadRequest(c, "invalid request body")
		return
	}
	partyID, err := uuid.Parse(body.PartyID)
	if err != nil {
		writeBadRequest(c, "invalid party_id")
		return
	}
	source := domain.OptInSourceManual
	if body.Source != "" {
		source = domain.OptInSource(body.Source)
	}
	optIn, err := h.uc.RegisterOptIn(c.Request.Context(), tenantID, partyID, body.Phone, source)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toOptInResponse(optIn))
}

func (h *Handler) RegisterOptOut(c *gin.Context) {
	tenantID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	partyID, err := uuid.Parse(c.Param("party_id"))
	if err != nil {
		writeBadRequest(c, "invalid party_id")
		return
	}
	if err := h.uc.RegisterOptOut(c.Request.Context(), tenantID, partyID); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) CheckOptIn(c *gin.Context) {
	tenantID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	partyID, err := uuid.Parse(c.Param("party_id"))
	if err != nil {
		writeBadRequest(c, "invalid party_id")
		return
	}
	optedIn, err := h.uc.IsOptedIn(c.Request.Context(), tenantID, partyID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"opted_in": optedIn})
}
