package org

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/org/handler/dto"
	orgdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/org/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/control-plane/backend/pkg/http/errors"
)

type usecasesPort interface {
	Create(ctx context.Context, name, slug, externalID, actor string) (orgdomain.Organization, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/orgs", h.CreateOrg)
}

func (h *Handler) CreateOrg(c *gin.Context) {
	var req dto.CreateOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	org, err := h.uc.Create(c.Request.Context(), req.Name, req.Slug, req.ExternalID, req.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, org)
}
