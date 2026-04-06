package services

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	crudpaths "github.com/devpablocristo/modules/crud/paths/go/paths"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/services/handler/dto"
	servicedomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/services/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]servicedomain.Service, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in servicedomain.Service, actor string) (servicedomain.Service, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (servicedomain.Service, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (servicedomain.Service, error)
	Archive(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Delete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	const servicesBasePath = "/services"
	const servicesItemPath = servicesBasePath + "/:id"

	auth.GET(servicesBasePath, rbac.RequirePermission("services", "read"), h.List)
	auth.POST(servicesBasePath, rbac.RequirePermission("services", "create"), h.Create)
	auth.GET(servicesItemPath, rbac.RequirePermission("services", "read"), h.Get)
	auth.PATCH(servicesItemPath, rbac.RequirePermission("services", "update"), h.Update)
	auth.DELETE(servicesItemPath, rbac.RequirePermission("services", "delete"), h.Delete)
	auth.POST(servicesItemPath+"/"+crudpaths.SegmentArchive, rbac.RequirePermission("services", "update"), h.Archive)
	auth.POST(servicesItemPath+"/"+crudpaths.SegmentRestore, rbac.RequirePermission("services", "update"), h.Restore)
}

func (h *Handler) List(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		writeValidation(c, "invalid org")
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	after, ok := handlers.ParseAfterUUIDQuery(c)
	if !ok {
		return
	}
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{
		OrgID:    orgID,
		Limit:    limit,
		After:    after,
		Search:   c.Query("search"),
		Tag:      c.Query("tag"),
		Sort:     c.Query("sort"),
		Order:    c.Query("order"),
		Archived: strings.EqualFold(strings.TrimSpace(c.Query("archived")), "true"),
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListServicesResponse{Items: make([]dto.ServiceItem, 0, len(items)), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toServiceItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		writeValidation(c, "invalid org")
		return
	}
	var req dto.CreateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeValidation(c, "invalid request body")
		return
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	out, err := h.uc.Create(c.Request.Context(), servicedomain.Service{
		OrgID:                  orgID,
		Code:                   req.Code,
		Name:                   req.Name,
		Description:            req.Description,
		CategoryCode:           req.CategoryCode,
		SalePrice:              req.SalePrice,
		CostPrice:              req.CostPrice,
		TaxRate:                req.TaxRate,
		Currency:               req.Currency,
		DefaultDurationMinutes: req.DefaultDurationMinutes,
		IsActive:               isActive,
		Tags:                   req.Tags,
		Metadata: func() map[string]any {
			if req.Metadata == nil {
				return map[string]any{}
			}
			return req.Metadata
		}(),
	}, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toServiceItem(out))
}

func (h *Handler) Get(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		writeValidation(c, "invalid org")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeValidation(c, "invalid id")
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toServiceItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		writeValidation(c, "invalid org")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeValidation(c, "invalid id")
		return
	}
	var req dto.UpdateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeValidation(c, "invalid request body")
		return
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		Code:                   req.Code,
		Name:                   req.Name,
		Description:            req.Description,
		CategoryCode:           req.CategoryCode,
		SalePrice:              req.SalePrice,
		CostPrice:              req.CostPrice,
		TaxRate:                req.TaxRate,
		Currency:               req.Currency,
		DefaultDurationMinutes: req.DefaultDurationMinutes,
		IsActive:               req.IsActive,
		Tags:                   req.Tags,
		Metadata:               req.Metadata,
	}, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toServiceItem(out))
}

func (h *Handler) Delete(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		writeValidation(c, "invalid org")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeValidation(c, "invalid id")
		return
	}
	if err := h.uc.Delete(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Archive(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		writeValidation(c, "invalid org")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeValidation(c, "invalid id")
		return
	}
	if err := h.uc.Archive(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Restore(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		writeValidation(c, "invalid org")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeValidation(c, "invalid id")
		return
	}
	if err := h.uc.Restore(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func toServiceItem(in servicedomain.Service) dto.ServiceItem {
	return dto.ServiceItem{
		ID:                     in.ID.String(),
		OrgID:                  in.OrgID.String(),
		Code:                   in.Code,
		Name:                   in.Name,
		Description:            in.Description,
		CategoryCode:           in.CategoryCode,
		SalePrice:              in.SalePrice,
		CostPrice:              in.CostPrice,
		TaxRate:                in.TaxRate,
		Currency:               in.Currency,
		DefaultDurationMinutes: in.DefaultDurationMinutes,
		IsActive:               in.IsActive,
		Tags:                   in.Tags,
		Metadata:               in.Metadata,
		CreatedAt:              in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:              in.UpdatedAt.UTC().Format(time.RFC3339),
		DeletedAt:              formatOptionalTime(in.DeletedAt),
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func writeValidation(c *gin.Context, message string) {
	httperrors.Write(c, http.StatusBadRequest, "VALIDATION", message)
}
