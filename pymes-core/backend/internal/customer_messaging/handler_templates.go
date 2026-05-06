package customer_messaging

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/handler/dto"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

func (h *Handler) ListTemplates(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	tpls, err := h.uc.ListTemplates(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	items := make([]dto.TemplateResponse, 0, len(tpls))
	for _, t := range tpls {
		items = append(items, toTemplateResponse(t))
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateTemplate(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	var body dto.CreateTemplateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		writeBadRequest(c, "invalid request body")
		return
	}
	buttons := make([]domain.TemplateButton, 0, len(body.Buttons))
	for _, b := range body.Buttons {
		buttons = append(buttons, domain.TemplateButton{Type: b.Type, Text: b.Text, URL: b.URL, Phone: b.Phone, Payload: b.Payload})
	}
	tpl, err := h.uc.CreateTemplate(c.Request.Context(), orgID, domain.Template{Name: body.Name, Language: body.Language, Category: domain.TemplateCategory(body.Category), HeaderType: body.HeaderType, HeaderText: body.HeaderText, BodyText: body.BodyText, FooterText: body.FooterText, Buttons: buttons})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toTemplateResponse(tpl))
}

func (h *Handler) GetTemplate(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	tpl, err := h.uc.GetTemplate(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toTemplateResponse(tpl))
}

func (h *Handler) DeleteTemplate(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.DeleteTemplate(c.Request.Context(), orgID, id); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
