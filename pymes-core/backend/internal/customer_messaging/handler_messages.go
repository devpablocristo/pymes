package customer_messaging

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/handler/dto"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

func (h *Handler) SendText(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	var body dto.SendTextRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		writeBadRequest(c, "invalid request body")
		return
	}
	partyID, err := uuid.Parse(body.PartyID)
	if err != nil {
		writeBadRequest(c, "invalid party_id")
		return
	}
	msg, err := h.uc.SendText(c.Request.Context(), domain.SendTextRequest{OrgID: orgID, PartyID: partyID, Body: body.Body, Actor: auth.Actor})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toMessageResponse(msg))
}

func (h *Handler) SendTemplate(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	var body dto.SendTemplateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		writeBadRequest(c, "invalid request body")
		return
	}
	partyID, err := uuid.Parse(body.PartyID)
	if err != nil {
		writeBadRequest(c, "invalid party_id")
		return
	}
	msg, err := h.uc.SendTemplate(c.Request.Context(), domain.SendTemplateRequest{OrgID: orgID, PartyID: partyID, TemplateName: body.TemplateName, Language: body.Language, Params: body.Params, Actor: auth.Actor})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toMessageResponse(msg))
}

func (h *Handler) SendMedia(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	var body dto.SendMediaRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		writeBadRequest(c, "invalid request body")
		return
	}
	partyID, err := uuid.Parse(body.PartyID)
	if err != nil {
		writeBadRequest(c, "invalid party_id")
		return
	}
	msg, err := h.uc.SendMedia(c.Request.Context(), domain.SendMediaRequest{OrgID: orgID, PartyID: partyID, MediaType: domain.MessageType(body.MediaType), MediaURL: body.MediaURL, Caption: body.Caption, Actor: auth.Actor})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toMessageResponse(msg))
}

func (h *Handler) SendInteractive(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	var body dto.SendInteractiveRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		writeBadRequest(c, "invalid request body")
		return
	}
	partyID, err := uuid.Parse(body.PartyID)
	if err != nil {
		writeBadRequest(c, "invalid party_id")
		return
	}
	buttons := make([]domain.InteractiveButton, 0, len(body.Buttons))
	for _, b := range body.Buttons {
		buttons = append(buttons, domain.InteractiveButton{ID: b.ID, Title: b.Title})
	}
	msg, err := h.uc.SendInteractive(c.Request.Context(), domain.SendInteractiveRequest{OrgID: orgID, PartyID: partyID, Body: body.Body, Buttons: buttons, Actor: auth.Actor})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toMessageResponse(msg))
}

func (h *Handler) ListMessages(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	filter := domain.MessageFilter{OrgID: orgID}
	if pid := strings.TrimSpace(c.Query("party_id")); pid != "" {
		id, err := uuid.Parse(pid)
		if err != nil {
			writeBadRequest(c, "invalid party_id")
			return
		}
		filter.PartyID = &id
	}
	if d := strings.TrimSpace(c.Query("direction")); d != "" {
		dir := domain.MessageDirection(d)
		filter.Direction = &dir
	}
	if s := strings.TrimSpace(c.Query("status")); s != "" {
		st := domain.MessageStatus(s)
		filter.Status = &st
	}
	if l := strings.TrimSpace(c.Query("limit")); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			filter.Limit = v
		}
	}
	if o := strings.TrimSpace(c.Query("offset")); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			filter.Offset = v
		}
	}
	messages, total, err := h.uc.ListMessages(c.Request.Context(), filter)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	items := make([]dto.MessageResponse, 0, len(messages))
	for _, m := range messages {
		items = append(items, toMessageResponse(m))
	}
	c.JSON(http.StatusOK, dto.MessageListResponse{Messages: items, Total: total})
}
