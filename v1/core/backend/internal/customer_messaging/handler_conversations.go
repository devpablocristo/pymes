package customer_messaging

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/core/backend/internal/customer_messaging/domain"
	"github.com/devpablocristo/pymes/core/backend/internal/customer_messaging/handler/dto"
	"github.com/devpablocristo/pymes/core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/core/shared/backend/httperrors"
)

func (h *Handler) ListWAConversations(c *gin.Context) {
	orgID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	convs, err := h.uc.ListConversations(c.Request.Context(), orgID, c.Query("assigned_to"), c.Query("status"), 100)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	items := make([]dto.ConversationResponse, 0, len(convs))
	for _, conv := range convs {
		items = append(items, conversationToDTO(&conv))
	}
	c.JSON(http.StatusOK, dto.ConversationListResponse{Items: items})
}

func (h *Handler) AssignWAConversation(c *gin.Context) {
	orgID, convID, ok := handlers.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var body dto.AssignConversationRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		writeBadRequest(c, "invalid request body")
		return
	}
	if err := h.uc.AssignConversation(c.Request.Context(), orgID, convID, body.AssignedTo); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "assigned"})
}

func (h *Handler) MarkWAConversationRead(c *gin.Context) {
	orgID, convID, ok := handlers.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.MarkConversationRead(c.Request.Context(), orgID, convID); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "read"})
}

func (h *Handler) ResolveWAConversation(c *gin.Context) {
	orgID, convID, ok := handlers.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.ResolveConversation(c.Request.Context(), orgID, convID); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "resolved"})
}

func conversationToDTO(c *domain.Conversation) dto.ConversationResponse {
	r := dto.ConversationResponse{ID: c.ID, PartyID: c.PartyID, Phone: c.Phone, PartyName: c.PartyName, AssignedTo: c.AssignedTo, Status: string(c.Status), LastMessagePreview: c.LastMessagePreview, UnreadCount: c.UnreadCount, CreatedAt: c.CreatedAt.Format(timeFmt), UpdatedAt: c.UpdatedAt.Format(timeFmt)}
	if c.LastMessageAt != nil {
		v := c.LastMessageAt.Format(timeFmt)
		r.LastMessageAt = &v
	}
	return r
}
