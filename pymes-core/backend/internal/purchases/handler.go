package purchases

import (
	"context"
	"net/http"
	"strings"

	"github.com/devpablocristo/core/http/go/pagination"
	crudpaths "github.com/devpablocristo/modules/crud/paths/go/paths"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/purchases/handler/dto"
	purchasesdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/purchases/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, status string, limit int) ([]purchasesdomain.Purchase, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, status string, limit int) ([]purchasesdomain.Purchase, error)
	Create(ctx context.Context, in CreateInput) (purchasesdomain.Purchase, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (purchasesdomain.Purchase, error)
	Update(ctx context.Context, in UpdateInput, actor string) (purchasesdomain.Purchase, error)
	UpdateStatus(ctx context.Context, in UpdateStatusInput, actor string) (purchasesdomain.Purchase, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/purchases", rbac.RequirePermission("purchases", "read"), h.List)
	auth.GET("/purchases/"+crudpaths.SegmentArchived, rbac.RequirePermission("purchases", "read"), h.ListArchived)
	auth.POST("/purchases", rbac.RequirePermission("purchases", "create"), h.Create)
	auth.GET("/purchases/:id", rbac.RequirePermission("purchases", "read"), h.Get)
	auth.PATCH("/purchases/:id", rbac.RequirePermission("purchases", "update"), h.Update)
	auth.PATCH("/purchases/:id/status", rbac.RequirePermission("purchases", "update"), h.UpdateStatus)
	auth.DELETE("/purchases/:id", rbac.RequirePermission("purchases", "delete"), h.Delete)
	auth.POST("/purchases/:id/"+crudpaths.SegmentRestore, rbac.RequirePermission("purchases", "delete"), h.Restore)
	auth.DELETE("/purchases/:id/"+crudpaths.SegmentHard, rbac.RequirePermission("purchases", "delete"), h.HardDelete)
}

func (h *Handler) List(c *gin.Context) {
	orgID, ok := parseOrg(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	var branchID *uuid.UUID
	if v := strings.TrimSpace(c.Query("branch_id")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
			return
		}
		branchID = &id
	}
	items, err := h.uc.List(c.Request.Context(), orgID, branchID, c.Query("status"), limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) ListArchived(c *gin.Context) {
	orgID, ok := parseOrg(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	var branchID *uuid.UUID
	if v := strings.TrimSpace(c.Query("branch_id")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
			return
		}
		branchID = &id
	}
	items, err := h.uc.ListArchived(c.Request.Context(), orgID, branchID, c.Query("status"), limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) Create(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	var req dto.CreatePurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	payload, err := buildCreateInput(orgID, req, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	out, err := h.uc.Create(c.Request.Context(), payload)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) Get(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Update(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	var req dto.CreatePurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	payload, err := buildCreateInput(orgID, req, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	out, err := h.uc.Update(c.Request.Context(), UpdateInput{ID: id, OrgID: orgID, BranchID: payload.BranchID, SupplierID: payload.SupplierID, SupplierName: payload.SupplierName, Status: payload.Status, PaymentStatus: payload.PaymentStatus, IsFavorite: payload.IsFavorite, Tags: payload.Tags, Notes: payload.Notes, Items: payload.Items}, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) UpdateStatus(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	var req dto.UpdatePurchaseStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.UpdateStatus(c.Request.Context(), UpdateStatusInput{
		ID:     id,
		OrgID:  orgID,
		Status: strings.TrimSpace(req.Status),
	}, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Delete(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	if err := h.uc.SoftDelete(c.Request.Context(), orgID, id, authCtx.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Restore(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	if err := h.uc.Restore(c.Request.Context(), orgID, id, authCtx.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HardDelete(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	if err := h.uc.HardDelete(c.Request.Context(), orgID, id, authCtx.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func buildCreateInput(orgID uuid.UUID, req dto.CreatePurchaseRequest, actor string) (CreateInput, error) {
	var branchID *uuid.UUID
	if req.BranchID != nil && strings.TrimSpace(*req.BranchID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*req.BranchID))
		if err != nil {
			return CreateInput{}, httperrors.ErrBadInput
		}
		branchID = &parsed
	}
	var supplierID *uuid.UUID
	if req.SupplierID != nil && strings.TrimSpace(*req.SupplierID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*req.SupplierID))
		if err != nil {
			return CreateInput{}, httperrors.ErrBadInput
		}
		supplierID = &parsed
	}
	items := make([]purchasesdomain.PurchaseItem, 0, len(req.Items))
	for _, item := range req.Items {
		var productID *uuid.UUID
		if item.ProductID != nil && strings.TrimSpace(*item.ProductID) != "" {
			parsed, err := uuid.Parse(strings.TrimSpace(*item.ProductID))
			if err != nil {
				return CreateInput{}, httperrors.ErrBadInput
			}
			productID = &parsed
		}
		var serviceID *uuid.UUID
		if item.ServiceID != nil && strings.TrimSpace(*item.ServiceID) != "" {
			parsed, err := uuid.Parse(strings.TrimSpace(*item.ServiceID))
			if err != nil {
				return CreateInput{}, httperrors.ErrBadInput
			}
			serviceID = &parsed
		}
		taxRate := 0.0
		if item.TaxRate != nil {
			taxRate = *item.TaxRate
		}
		items = append(items, purchasesdomain.PurchaseItem{ProductID: productID, ServiceID: serviceID, Description: strings.TrimSpace(item.Description), Quantity: item.Quantity, UnitCost: item.UnitCost, TaxRate: taxRate})
	}
	isFavorite := req.IsFavorite != nil && *req.IsFavorite
	return CreateInput{OrgID: orgID, BranchID: branchID, SupplierID: supplierID, SupplierName: strings.TrimSpace(req.SupplierName), Status: strings.TrimSpace(req.Status), PaymentStatus: strings.TrimSpace(req.PaymentStatus), IsFavorite: isFavorite, Tags: req.Tags, Notes: strings.TrimSpace(req.Notes), CreatedBy: actor, Items: items}, nil
}

func parseOrg(c *gin.Context) (uuid.UUID, bool) {
	return handlers.ParseAuthOrgID(c)
}
func parseOrgID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	return handlers.ParseAuthOrgAndParamID(c, "id", "id")
}
