package sales

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/sales/handler/dto"
	saledomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/sales/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]saledomain.Sale, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in CreateSaleInput) (saledomain.Sale, error)
	GetByID(ctx context.Context, orgID, saleID uuid.UUID) (saledomain.Sale, error)
	PatchSale(ctx context.Context, orgID, saleID uuid.UUID, in SalePatchFields, actor string) (saledomain.Sale, error)
	Void(ctx context.Context, orgID, saleID uuid.UUID, actor string) (saledomain.Sale, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/sales", rbac.RequirePermission("sales", "read"), h.List)
	auth.POST("/sales", rbac.RequirePermission("sales", "create"), h.Create)
	auth.GET("/sales/:id", rbac.RequirePermission("sales", "read"), h.Get)
	auth.PATCH("/sales/:id", rbac.RequirePermission("sales", "update"), h.Patch)
	auth.POST("/sales/:id/void", rbac.RequirePermission("sales", "void"), h.Void)
}

func (h *Handler) List(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}

	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	after, ok := handlers.ParseAfterUUIDQuery(c)
	if !ok {
		return
	}
	var customerID *uuid.UUID
	var branchID *uuid.UUID
	if v := strings.TrimSpace(c.Query("branch_id")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
			return
		}
		branchID = &id
	}
	if v := strings.TrimSpace(c.Query("customer_id")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer_id"})
			return
		}
		customerID = &id
	}

	from, err := parseDatePtr(c.Query("from"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from"})
		return
	}
	to, err := parseDatePtr(c.Query("to"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to"})
		return
	}

	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{
		OrgID:         orgID,
		BranchID:      branchID,
		Limit:         limit,
		After:         after,
		CustomerID:    customerID,
		PaymentMethod: c.Query("payment_method"),
		From:          from,
		To:            to,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}

	resp := dto.ListSalesResponse{
		Items:   make([]dto.SaleResponse, 0, len(items)),
		Total:   total,
		HasMore: hasMore,
	}
	if next != nil {
		resp.NextCursor = next.String()
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toSaleResponse(item))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}

	var req dto.CreateSaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	var customerID *uuid.UUID
	var branchID *uuid.UUID
	if req.CustomerID != nil && strings.TrimSpace(*req.CustomerID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.CustomerID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer_id"})
			return
		}
		customerID = &id
	}
	var quoteID *uuid.UUID
	if req.QuoteID != nil && strings.TrimSpace(*req.QuoteID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.QuoteID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quote_id"})
			return
		}
		quoteID = &id
	}
	if req.BranchID != nil && strings.TrimSpace(*req.BranchID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.BranchID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
			return
		}
		branchID = &id
	}

	items := make([]CreateSaleItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		var productID *uuid.UUID
		if it.ProductID != nil && strings.TrimSpace(*it.ProductID) != "" {
			id, err := uuid.Parse(strings.TrimSpace(*it.ProductID))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id"})
				return
			}
			productID = &id
		}
		var serviceID *uuid.UUID
		if it.ServiceID != nil && strings.TrimSpace(*it.ServiceID) != "" {
			id, err := uuid.Parse(strings.TrimSpace(*it.ServiceID))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service_id"})
				return
			}
			serviceID = &id
		}
		items = append(items, CreateSaleItemInput{
			ProductID:   productID,
			ServiceID:   serviceID,
			Description: it.Description,
			Quantity:    it.Quantity,
			UnitPrice:   it.UnitPrice,
			TaxRate:     it.TaxRate,
			SortOrder:   it.SortOrder,
		})
	}

	out, err := h.uc.Create(c.Request.Context(), CreateSaleInput{
		OrgID:         orgID,
		BranchID:      branchID,
		CustomerID:    customerID,
		CustomerName:  req.CustomerName,
		QuoteID:       quoteID,
		PaymentMethod: req.PaymentMethod,
		Items:         items,
		Notes:         req.Notes,
		Tags:          req.Tags,
		Metadata:      req.Metadata,
		CreatedBy:     a.Actor,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toSaleResponse(out))
}

func (h *Handler) Get(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toSaleResponse(out))
}

func (h *Handler) Patch(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req dto.PatchSaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Tags == nil && req.Metadata == nil && req.Notes == nil && req.PaymentMethod == nil && req.CustomerName == nil && req.BranchID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no patch fields"})
		return
	}

	var branchID *uuid.UUID
	if req.BranchID != nil && strings.TrimSpace(*req.BranchID) != "" {
		bid, err := uuid.Parse(strings.TrimSpace(*req.BranchID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
			return
		}
		branchID = &bid
	}

	var pm *string
	if req.PaymentMethod != nil {
		t := strings.TrimSpace(*req.PaymentMethod)
		pm = &t
	}

	out, err := h.uc.PatchSale(c.Request.Context(), orgID, id, SalePatchFields{
		Tags:          req.Tags,
		Metadata:      req.Metadata,
		Notes:         req.Notes,
		PaymentMethod: pm,
		CustomerName:  req.CustomerName,
		BranchID:      branchID,
	}, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toSaleResponse(out))
}

func (h *Handler) Void(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	out, err := h.uc.Void(c.Request.Context(), orgID, id, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toSaleResponse(out))
}

func toSaleResponse(in saledomain.Sale) dto.SaleResponse {
	resp := dto.SaleResponse{
		ID:            in.ID.String(),
		OrgID:         in.OrgID.String(),
		Number:        in.Number,
		CustomerName:  in.CustomerName,
		Status:        in.Status,
		PaymentMethod: in.PaymentMethod,
		Items:         make([]dto.SaleItemResponse, 0, len(in.Items)),
		Subtotal:      in.Subtotal,
		TaxTotal:      in.TaxTotal,
		Total:         in.Total,
		Currency:      in.Currency,
		Notes:         in.Notes,
		CreatedBy:     in.CreatedBy,
		CreatedAt:     in.CreatedAt.UTC().Format(time.RFC3339),
		Tags:          in.Tags,
		Metadata:      in.Metadata,
	}
	if in.BranchID != nil {
		resp.BranchID = in.BranchID.String()
	}
	if in.CustomerID != nil {
		resp.CustomerID = in.CustomerID.String()
	}
	if in.QuoteID != nil {
		resp.QuoteID = in.QuoteID.String()
	}
	for _, item := range in.Items {
		out := dto.SaleItemResponse{
			ID:          item.ID.String(),
			SaleID:      item.SaleID.String(),
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			CostPrice:   item.CostPrice,
			TaxRate:     item.TaxRate,
			Subtotal:    item.Subtotal,
			SortOrder:   item.SortOrder,
		}
		if item.ProductID != nil {
			out.ProductID = item.ProductID.String()
		}
		if item.ServiceID != nil {
			out.ServiceID = item.ServiceID.String()
		}
		resp.Items = append(resp.Items, out)
	}
	return resp
}

func parseDatePtr(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, err
	}
	t = t.UTC()
	return &t, nil
}
