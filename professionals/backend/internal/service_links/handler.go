package service_links

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/professionals/backend/internal/service_links/handler/dto"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/service_links/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pkgs/go-pkg/httperrors"
	"github.com/devpablocristo/pymes/pkgs/go-pkg/auth"
)

type usecasesPort interface {
	ListByProfile(ctx context.Context, orgID, profileID uuid.UUID) ([]domain.ServiceLink, error)
	ReplaceForProfile(ctx context.Context, orgID, profileID uuid.UUID, links []domain.ServiceLink, actor string) ([]domain.ServiceLink, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.GET("/professionals/:id/services", h.List)
	authGroup.PUT("/professionals/:id/services", h.Replace)
}

func (h *Handler) List(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	profileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	items, err := h.uc.ListByProfile(c.Request.Context(), orgID, profileID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListServiceLinksResponse{Items: make([]dto.ServiceLinkItem, 0, len(items))}
	for _, it := range items {
		resp.Items = append(resp.Items, toServiceLinkItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Replace(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	profileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req dto.ReplaceServiceLinksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	links := make([]domain.ServiceLink, 0, len(req.Links))
	for _, l := range req.Links {
		productID, err := uuid.Parse(l.ProductID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id: " + l.ProductID})
			return
		}
		meta := l.Metadata
		if meta == nil {
			meta = map[string]any{}
		}
		links = append(links, domain.ServiceLink{
			ProductID:         productID,
			PublicDescription: l.PublicDescription,
			DisplayOrder:      l.DisplayOrder,
			IsFeatured:        l.IsFeatured,
			Metadata:          meta,
		})
	}
	items, err := h.uc.ReplaceForProfile(c.Request.Context(), orgID, profileID, links, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListServiceLinksResponse{Items: make([]dto.ServiceLinkItem, 0, len(items))}
	for _, it := range items {
		resp.Items = append(resp.Items, toServiceLinkItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func toServiceLinkItem(in domain.ServiceLink) dto.ServiceLinkItem {
	return dto.ServiceLinkItem{
		ID:                in.ID.String(),
		OrgID:             in.OrgID.String(),
		ProfileID:         in.ProfileID.String(),
		ProductID:         in.ProductID.String(),
		PublicDescription: in.PublicDescription,
		DisplayOrder:      in.DisplayOrder,
		IsFeatured:        in.IsFeatured,
		Metadata:          in.Metadata,
		CreatedAt:         in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         in.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
