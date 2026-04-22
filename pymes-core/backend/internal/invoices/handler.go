package invoices

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	crudpaths "github.com/devpablocristo/modules/crud/paths/go/paths"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/invoices/handler/dto"
	invdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/invoices/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]invdomain.Invoice, int64, bool, *uuid.UUID, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]invdomain.Invoice, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (invdomain.Invoice, error)
	Create(ctx context.Context, in CreateInput) (invdomain.Invoice, error)
	Update(ctx context.Context, in UpdateInput) (invdomain.Invoice, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	const base = "/invoices"
	const item = base + "/:id"

	auth.GET(base, rbac.RequirePermission("invoices", "read"), h.List)
	auth.GET(base+"/"+crudpaths.SegmentArchived, rbac.RequirePermission("invoices", "read"), h.ListArchived)
	auth.POST(base, rbac.RequirePermission("invoices", "create"), h.Create)
	auth.GET(item, rbac.RequirePermission("invoices", "read"), h.Get)
	auth.PATCH(item, rbac.RequirePermission("invoices", "update"), h.Update)
	auth.DELETE(item, rbac.RequirePermission("invoices", "delete"), h.Delete)
	auth.POST(item+"/"+crudpaths.SegmentArchive, rbac.RequirePermission("invoices", "update"), h.Delete)
	auth.POST(item+"/"+crudpaths.SegmentRestore, rbac.RequirePermission("invoices", "update"), h.RestoreAction)
	auth.DELETE(item+"/"+crudpaths.SegmentHard, rbac.RequirePermission("invoices", "delete"), h.HardDelete)
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
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{
		OrgID:  orgID,
		Limit:  limit,
		After:  after,
		Status: c.Query("status"),
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListInvoicesResponse{Items: make([]dto.InvoiceResponse, 0, len(items)), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toInvoiceResponse(it))
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
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, err := h.uc.ListArchived(c.Request.Context(), orgID, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListInvoicesResponse{Items: make([]dto.InvoiceResponse, 0, len(items)), Total: int64(len(items)), HasMore: false}
	for _, it := range items {
		resp.Items = append(resp.Items, toInvoiceResponse(it))
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
	var req dto.CreateInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	var partyID *uuid.UUID
	if req.PartyID != nil && strings.TrimSpace(*req.PartyID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.PartyID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid party_id"})
			return
		}
		partyID = &id
	}
	isFavorite := false
	if req.IsFavorite != nil {
		isFavorite = *req.IsFavorite
	}
	items := make([]CreateItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, CreateItemInput{
			Description: it.Description,
			Qty:         it.Qty,
			Unit:        it.Unit,
			UnitPrice:   it.UnitPrice,
			SortOrder:   it.SortOrder,
		})
	}
	out, err := h.uc.Create(c.Request.Context(), CreateInput{
		OrgID:           orgID,
		Number:          req.Number,
		PartyID:         partyID,
		CustomerName:    req.CustomerName,
		IssuedDate:      req.IssuedDate,
		DueDate:         req.DueDate,
		Status:          req.Status,
		DiscountPercent: req.DiscountPercent,
		TaxPercent:      req.TaxPercent,
		Notes:           req.Notes,
		IsFavorite:      isFavorite,
		Tags:            req.Tags,
		Items:           items,
		CreatedBy:       a.Actor,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toInvoiceResponse(out))
}

func (h *Handler) Get(c *gin.Context) {
	orgID, id, ok := parseOrgAndID(c)
	if !ok {
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toInvoiceResponse(out))
}

func (h *Handler) Update(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgAndID(c)
	if !ok {
		return
	}
	var req dto.UpdateInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.Update(c.Request.Context(), UpdateInput{
		OrgID:           orgID,
		ID:              id,
		Status:          req.Status,
		DiscountPercent: req.DiscountPercent,
		TaxPercent:      req.TaxPercent,
		Notes:           req.Notes,
		IsFavorite:      req.IsFavorite,
		Tags:            req.Tags,
		IssuedDate:      req.IssuedDate,
		DueDate:         req.DueDate,
		Actor:           a.Actor,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toInvoiceResponse(out))
}

func (h *Handler) Delete(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgAndID(c)
	if !ok {
		return
	}
	if err := h.uc.SoftDelete(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) RestoreAction(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgAndID(c)
	if !ok {
		return
	}
	if err := h.uc.Restore(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HardDelete(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgAndID(c)
	if !ok {
		return
	}
	if err := h.uc.HardDelete(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func parseOrgAndID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, id, true
}

func toInvoiceResponse(in invdomain.Invoice) dto.InvoiceResponse {
	tags := in.Tags
	if tags == nil {
		tags = []string{}
	}
	items := make([]dto.LineItemResponse, 0, len(in.Items))
	for _, it := range in.Items {
		items = append(items, dto.LineItemResponse{
			ID:          it.ID.String(),
			InvoiceID:   it.InvoiceID.String(),
			Description: it.Description,
			Qty:         it.Qty,
			Unit:        it.Unit,
			UnitPrice:   it.UnitPrice,
			LineTotal:   it.LineTotal,
			SortOrder:   it.SortOrder,
		})
	}
	resp := dto.InvoiceResponse{
		ID:              in.ID.String(),
		OrgID:           in.OrgID.String(),
		Number:          in.Number,
		CustomerName:    in.CustomerName,
		IssuedDate:      in.IssuedDate.UTC().Format("2006-01-02"),
		DueDate:         in.DueDate.UTC().Format("2006-01-02"),
		Status:          string(in.Status),
		Subtotal:        in.Subtotal,
		DiscountPercent: in.DiscountPercent,
		TaxPercent:      in.TaxPercent,
		Total:           in.Total,
		Notes:           in.Notes,
		IsFavorite:      in.IsFavorite,
		Tags:            tags,
		CreatedBy:       in.CreatedBy,
		CreatedAt:       in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       in.UpdatedAt.UTC().Format(time.RFC3339),
		Items:           items,
	}
	if in.PartyID != nil {
		resp.PartyID = in.PartyID.String()
	}
	if in.ArchivedAt != nil {
		resp.ArchivedAt = in.ArchivedAt.UTC().Format(time.RFC3339)
	}
	return resp
}
