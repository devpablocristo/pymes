package customer_messaging

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/handler/dto"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

func (h *Handler) ListCampaigns(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	campaigns, err := h.uc.ListCampaigns(c.Request.Context(), orgID, 100)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	items := make([]dto.CampaignResponse, 0, len(campaigns))
	for _, item := range campaigns {
		items = append(items, campaignToDTO(&item))
	}
	c.JSON(http.StatusOK, dto.CampaignListResponse{Items: items})
}

func (h *Handler) CreateCampaign(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	var body dto.CreateCampaignRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		writeBadRequest(c, "invalid request body")
		return
	}
	campaign, err := h.uc.CreateCampaign(c.Request.Context(), orgID, body.Name, body.TemplateName, body.TemplateLanguage, body.TagFilter, auth.Actor, body.TemplateParams)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, campaignToDTO(campaign))
}

func (h *Handler) GetCampaignDetail(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	campaign, err := h.uc.GetCampaign(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	recipients, err := h.uc.GetCampaignRecipients(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	items := make([]dto.CampaignRecipientResponse, 0, len(recipients))
	for _, rec := range recipients {
		items = append(items, campaignRecipientToDTO(&rec))
	}
	c.JSON(http.StatusOK, dto.CampaignDetailResponse{CampaignResponse: campaignToDTO(campaign), Recipients: items})
}

func (h *Handler) SendCampaign(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.SendCampaign(c.Request.Context(), orgID, id); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "sent"})
}
