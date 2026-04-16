package quotes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	crudpaths "github.com/devpablocristo/modules/crud/paths/go/paths"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/quotes/handler/dto"
	quotedomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/quotes/usecases/domain"
	salesdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/sales/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]quotedomain.Quote, int64, bool, *uuid.UUID, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]quotedomain.Quote, error)
	Create(ctx context.Context, in CreateQuoteInput) (quotedomain.Quote, error)
	GetByID(ctx context.Context, orgID, quoteID uuid.UUID) (quotedomain.Quote, error)
	Update(ctx context.Context, in UpdateQuoteInput) (quotedomain.Quote, error)
	Archive(ctx context.Context, orgID, quoteID uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, quoteID uuid.UUID, actor string) error
	HardDelete(ctx context.Context, orgID, quoteID uuid.UUID, actor string) error
	Send(ctx context.Context, orgID, quoteID uuid.UUID, actor string) (quotedomain.Quote, error)
	Accept(ctx context.Context, orgID, quoteID uuid.UUID, actor string) (quotedomain.Quote, error)
	Reject(ctx context.Context, orgID, quoteID uuid.UUID, actor string) (quotedomain.Quote, error)
	ToSale(ctx context.Context, orgID, quoteID uuid.UUID, paymentMethod, notes, actor string) (salesdomain.Sale, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	const quotesBasePath = "/quotes"
	const quotesItemPath = quotesBasePath + "/:id"

	auth.GET(quotesBasePath, rbac.RequirePermission("quotes", "read"), h.List)
	auth.GET(quotesBasePath+"/"+crudpaths.SegmentArchived, rbac.RequirePermission("quotes", "read"), h.ListArchived)
	auth.POST(quotesBasePath, rbac.RequirePermission("quotes", "create"), h.Create)
	auth.GET(quotesItemPath, rbac.RequirePermission("quotes", "read"), h.Get)
	auth.PUT(quotesItemPath, rbac.RequirePermission("quotes", "update"), h.Update)
	auth.DELETE(quotesItemPath, rbac.RequirePermission("quotes", "delete"), h.Delete)
	auth.POST(quotesItemPath+"/"+crudpaths.SegmentRestore, rbac.RequirePermission("quotes", "delete"), h.Restore)
	auth.DELETE(quotesItemPath+"/"+crudpaths.SegmentHard, rbac.RequirePermission("quotes", "delete"), h.HardDelete)
	auth.POST(quotesItemPath+"/send", rbac.RequirePermission("quotes", "update"), h.Send)
	auth.POST(quotesItemPath+"/accept", rbac.RequirePermission("quotes", "update"), h.Accept)
	auth.POST(quotesItemPath+"/reject", rbac.RequirePermission("quotes", "update"), h.Reject)
	auth.POST(quotesItemPath+"/to-sale", rbac.RequirePermission("quotes", "update"), h.ToSale)
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
		OrgID:      orgID,
		BranchID:   branchID,
		Limit:      limit,
		After:      after,
		Status:     c.Query("status"),
		CustomerID: customerID,
		From:       from,
		To:         to,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListQuotesResponse{
		Items:   make([]dto.QuoteResponse, 0, len(items)),
		Total:   total,
		HasMore: hasMore,
	}
	if next != nil {
		resp.NextCursor = next.String()
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toQuoteResponse(item))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListArchived(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	var branchID *uuid.UUID
	if v := strings.TrimSpace(c.Query("branch_id")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
			return
		}
		branchID = &id
	}
	items, err := h.uc.ListArchived(c.Request.Context(), orgID, branchID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListQuotesResponse{
		Items:   make([]dto.QuoteResponse, 0, len(items)),
		Total:   int64(len(items)),
		HasMore: false,
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toQuoteResponse(item))
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

	var req dto.CreateQuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	customerID, err := parseOptionalUUID(req.CustomerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer_id"})
		return
	}
	var branchID *uuid.UUID
	if req.BranchID != nil && strings.TrimSpace(*req.BranchID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.BranchID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
			return
		}
		branchID = &id
	}
	validUntil, err := parseOptionalDate(req.ValidUntil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid valid_until"})
		return
	}
	items, err := parseItemInputs(req.Items)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	out, err := h.uc.Create(c.Request.Context(), CreateQuoteInput{
		OrgID:        orgID,
		BranchID:     branchID,
		CustomerID:   customerID,
		CustomerName: req.CustomerName,
		Items:        items,
		Notes:        req.Notes,
		ValidUntil:   validUntil,
		CreatedBy:    a.Actor,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toQuoteResponse(out))
}

func (h *Handler) Get(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	quoteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, quoteID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toQuoteResponse(out))
}

func (h *Handler) Update(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	quoteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req dto.UpdateQuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	var customerID **uuid.UUID
	if req.CustomerID != nil {
		parsed, err := parseOptionalUUID(req.CustomerID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer_id"})
			return
		}
		customerID = &parsed
	}
	var validUntil **time.Time
	if req.ValidUntil != nil {
		parsed, err := parseOptionalDate(req.ValidUntil)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid valid_until"})
			return
		}
		validUntil = &parsed
	}
	var items *[]QuoteItemInput
	if req.Items != nil {
		parsed, err := parseItemInputs(*req.Items)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		items = &parsed
	}

	out, err := h.uc.Update(c.Request.Context(), UpdateQuoteInput{
		OrgID:        orgID,
		ID:           quoteID,
		CustomerID:   customerID,
		CustomerName: req.CustomerName,
		Items:        items,
		Notes:        req.Notes,
		ValidUntil:   validUntil,
		Actor:        a.Actor,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toQuoteResponse(out))
}

func (h *Handler) Delete(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	quoteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.uc.Archive(c.Request.Context(), orgID, quoteID, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Restore(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	quoteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.uc.Restore(c.Request.Context(), orgID, quoteID, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HardDelete(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	quoteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.uc.HardDelete(c.Request.Context(), orgID, quoteID, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Send(c *gin.Context) {
	h.transition(c, "send")
}

func (h *Handler) Accept(c *gin.Context) {
	h.transition(c, "accept")
}

func (h *Handler) Reject(c *gin.Context) {
	h.transition(c, "reject")
}

func (h *Handler) transition(c *gin.Context, action string) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	quoteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var out quotedomain.Quote
	switch action {
	case "send":
		out, err = h.uc.Send(c.Request.Context(), orgID, quoteID, a.Actor)
	case "accept":
		out, err = h.uc.Accept(c.Request.Context(), orgID, quoteID, a.Actor)
	case "reject":
		out, err = h.uc.Reject(c.Request.Context(), orgID, quoteID, a.Actor)
	}
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toQuoteResponse(out))
}

func (h *Handler) ToSale(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	quoteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req dto.ToSaleRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	saleOut, err := h.uc.ToSale(c.Request.Context(), orgID, quoteID, req.PaymentMethod, req.Notes, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"quote_id": quoteID.String(),
		"sale_id":  saleOut.ID.String(),
		"number":   saleOut.Number,
		"status":   saleOut.Status,
	})
}

func toQuoteResponse(in quotedomain.Quote) dto.QuoteResponse {
	resp := dto.QuoteResponse{
		ID:           in.ID.String(),
		OrgID:        in.OrgID.String(),
		Number:       in.Number,
		CustomerName: in.CustomerName,
		Status:       in.Status,
		Items:        make([]dto.QuoteItemResponse, 0, len(in.Items)),
		Subtotal:     in.Subtotal,
		TaxTotal:     in.TaxTotal,
		Total:        in.Total,
		Currency:     in.Currency,
		Notes:        in.Notes,
		CreatedBy:    in.CreatedBy,
		CreatedAt:    in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    in.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if in.BranchID != nil {
		resp.BranchID = in.BranchID.String()
	}
	if in.CustomerID != nil {
		resp.CustomerID = in.CustomerID.String()
	}
	if in.ValidUntil != nil {
		resp.ValidUntil = in.ValidUntil.UTC().Format(time.RFC3339)
	}
	if in.ArchivedAt != nil {
		resp.ArchivedAt = in.ArchivedAt.UTC().Format(time.RFC3339)
	}
	for _, item := range in.Items {
		out := dto.QuoteItemResponse{
			ID:          item.ID.String(),
			QuoteID:     item.QuoteID.String(),
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
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

func parseItemInputs(items []dto.QuoteItemPayload) ([]QuoteItemInput, error) {
	out := make([]QuoteItemInput, 0, len(items))
	for _, item := range items {
		var productID *uuid.UUID
		if item.ProductID != nil && strings.TrimSpace(*item.ProductID) != "" {
			id, err := uuid.Parse(strings.TrimSpace(*item.ProductID))
			if err != nil {
				return nil, fmt.Errorf("invalid product_id")
			}
			productID = &id
		}
		var serviceID *uuid.UUID
		if item.ServiceID != nil && strings.TrimSpace(*item.ServiceID) != "" {
			id, err := uuid.Parse(strings.TrimSpace(*item.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("invalid service_id")
			}
			serviceID = &id
		}
		out = append(out, QuoteItemInput{
			ProductID:   productID,
			ServiceID:   serviceID,
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			TaxRate:     item.TaxRate,
			SortOrder:   item.SortOrder,
		})
	}
	return out, nil
}

func parseOptionalUUID(raw *string) (*uuid.UUID, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	id, err := uuid.Parse(strings.TrimSpace(*raw))
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func parseOptionalDate(raw *string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(*raw))
	if err != nil {
		// fallback yyyy-mm-dd
		t2, err2 := time.Parse("2006-01-02", strings.TrimSpace(*raw))
		if err2 != nil {
			return nil, err
		}
		t = t2
	}
	t = t.UTC()
	return &t, nil
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
