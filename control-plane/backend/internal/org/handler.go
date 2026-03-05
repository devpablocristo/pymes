package org

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/org/handler/dto"
)

type Handler struct {
	uc *Usecases
}

func NewHandler(uc *Usecases) *Handler {
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, org)
}
