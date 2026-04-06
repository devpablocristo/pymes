package service_links

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/service_links/handler/dto"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/service_links/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalgin"
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
	orgID, profileID, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
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
	orgID, profileID, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.ReplaceServiceLinksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	inputs := req.Links
	if inputs == nil {
		inputs = req.Items
	}
	if inputs == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "links or items is required"})
		return
	}
	links := make([]domain.ServiceLink, 0, len(inputs))
	for _, l := range inputs {
		serviceID, err := uuid.Parse(l.ServiceID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service_id: " + l.ServiceID})
			return
		}
		meta := l.Metadata
		if meta == nil {
			meta = map[string]any{}
		}
		links = append(links, domain.ServiceLink{
			ServiceID:         serviceID,
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
	item := dto.ServiceLinkItem{
		ID:                in.ID.String(),
		OrgID:             in.OrgID.String(),
		ProfileID:         in.ProfileID.String(),
		ServiceID:         in.ServiceID.String(),
		PublicDescription: in.PublicDescription,
		DisplayOrder:      in.DisplayOrder,
		IsFeatured:        in.IsFeatured,
		Metadata:          in.Metadata,
		CreatedAt:         in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         in.UpdatedAt.UTC().Format(time.RFC3339),
	}
	return item
}
