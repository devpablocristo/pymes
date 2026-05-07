package returns

import (
	"context"
	"net/http"
	"strings"

	"github.com/devpablocristo/core/http/go/pagination"
	crudpaths "github.com/devpablocristo/modules/crud/paths/go/paths"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	returndto "github.com/devpablocristo/pymes/pymes-core/backend/internal/returns/handler/dto"
	returndomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/returns/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, tenantID uuid.UUID, limit int) ([]returndomain.Return, error)
	ListArchived(ctx context.Context, tenantID uuid.UUID, limit int) ([]returndomain.Return, error)
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (returndomain.Return, error)
	Create(ctx context.Context, in CreateReturnInput) (returndomain.Return, *returndomain.CreditNote, error)
	Update(ctx context.Context, in returndomain.Return, actor string) (returndomain.Return, error)
	SoftDelete(ctx context.Context, tenantID, id uuid.UUID, actor string) error
	RestoreArchived(ctx context.Context, tenantID, id uuid.UUID, actor string) error
	HardDelete(ctx context.Context, tenantID, id uuid.UUID, actor string) error
	Void(ctx context.Context, tenantID, id uuid.UUID, actor string) (returndomain.Return, error)
	ListCreditNotes(ctx context.Context, tenantID uuid.UUID, partyID *uuid.UUID, limit int) ([]returndomain.CreditNote, error)
	GetCreditNote(ctx context.Context, tenantID, id uuid.UUID) (returndomain.CreditNote, error)
	ApplyCredit(ctx context.Context, in ApplyCreditInput) (returndomain.CreditNote, error)
	CreateManualCreditNote(ctx context.Context, in CreateManualCreditNoteInput) (returndomain.CreditNote, error)
}

type updateReturnRequest struct {
	Notes      *string   `json:"notes"`
	IsFavorite *bool     `json:"is_favorite"`
	Tags       *[]string `json:"tags"`
}

type createReturnRequestItem struct {
	SaleItemID string  `json:"sale_item_id" binding:"required"`
	Quantity   float64 `json:"quantity" binding:"required"`
}

type createReturnRequest struct {
	Reason       string                    `json:"reason"`
	RefundMethod string                    `json:"refund_method" binding:"required"`
	Notes        string                    `json:"notes"`
	Items        []createReturnRequestItem `json:"items" binding:"required"`
}

type applyCreditRequest struct {
	CreditNoteID string  `json:"credit_note_id" binding:"required"`
	Amount       float64 `json:"amount"`
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	const base = "/returns"
	const item = base + "/:id"

	auth.POST("/sales/:id/return", rbac.RequirePermission("returns", "create"), h.Create)
	auth.GET(base, rbac.RequirePermission("returns", "read"), h.List)
	auth.GET(base+"/"+crudpaths.SegmentArchived, rbac.RequirePermission("returns", "read"), h.ListArchived)
	auth.GET(item, rbac.RequirePermission("returns", "read"), h.Get)
	auth.PATCH(item, rbac.RequirePermission("returns", "update"), h.Update)
	auth.DELETE(item, rbac.RequirePermission("returns", "delete"), h.Delete)
	auth.POST(item+"/void", rbac.RequirePermission("returns", "create"), h.Void)
	auth.POST(item+"/"+crudpaths.SegmentArchive, rbac.RequirePermission("returns", "update"), h.Archive)
	auth.POST(item+"/"+crudpaths.SegmentRestore, rbac.RequirePermission("returns", "update"), h.Restore)
	auth.DELETE(item+"/"+crudpaths.SegmentHard, rbac.RequirePermission("returns", "delete"), h.HardDelete)

	auth.GET("/credit-notes", rbac.RequirePermission("returns", "read"), h.ListCreditNotes)
	auth.POST("/credit-notes", rbac.RequirePermission("returns", "create"), h.CreateCreditNote)
	auth.GET("/credit-notes/:id", rbac.RequirePermission("returns", "read"), h.GetCreditNote)
	auth.GET("/parties/:id/credit-notes", rbac.RequirePermission("returns", "read"), h.ListPartyCreditNotes)
	auth.POST("/sales/:id/apply-credit", rbac.RequirePermission("payments", "create"), h.ApplyCredit)
}

func (h *Handler) Create(c *gin.Context) {
	tenantID, saleID, ok := parseTenantSale(c)
	if !ok {
		return
	}
	var req createReturnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handlers.WriteValidation(c, "invalid request body")
		return
	}
	auth := handlers.GetAuthContext(c)
	items := make([]CreateReturnItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		saleItemID, err := uuid.Parse(strings.TrimSpace(item.SaleItemID))
		if err != nil {
			handlers.WriteValidation(c, "invalid sale_item_id")
			return
		}
		items = append(items, CreateReturnItemInput{SaleItemID: saleItemID, Quantity: item.Quantity})
	}
	out, credit, err := h.uc.Create(c.Request.Context(), CreateReturnInput{TenantID: tenantID, SaleID: saleID, Reason: req.Reason, RefundMethod: strings.TrimSpace(req.RefundMethod), Notes: strings.TrimSpace(req.Notes), CreatedBy: auth.Actor, Items: items})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"return": out, "credit_note": credit})
}

func (h *Handler) List(c *gin.Context) {
	tenantID, ok := parseTenant(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, err := h.uc.List(c.Request.Context(), tenantID, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	handlers.WriteOffsetListResponse(c, items, limit, len(items))
}

func (h *Handler) Get(c *gin.Context) {
	tenantID, id, ok := parseTenantAndID(c)
	if !ok {
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), tenantID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ListArchived(c *gin.Context) {
	tenantID, ok := parseTenant(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, err := h.uc.ListArchived(c.Request.Context(), tenantID, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	handlers.WriteOffsetListResponse(c, items, limit, len(items))
}

func (h *Handler) Update(c *gin.Context) {
	tenantID, id, ok := parseTenantAndID(c)
	if !ok {
		return
	}
	var req updateReturnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handlers.WriteValidation(c, "invalid request body")
		return
	}
	current, err := h.uc.GetByID(c.Request.Context(), tenantID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	if req.Notes != nil {
		current.Notes = *req.Notes
	}
	if req.IsFavorite != nil {
		current.IsFavorite = *req.IsFavorite
	}
	if req.Tags != nil {
		current.Tags = *req.Tags
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.Update(c.Request.Context(), current, auth.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// Delete realiza soft delete (archiva).
func (h *Handler) Delete(c *gin.Context) {
	tenantID, id, ok := parseTenantAndID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	if err := h.uc.SoftDelete(c.Request.Context(), tenantID, id, auth.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Archive(c *gin.Context) {
	h.Delete(c)
}

func (h *Handler) Restore(c *gin.Context) {
	tenantID, id, ok := parseTenantAndID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	if err := h.uc.RestoreArchived(c.Request.Context(), tenantID, id, auth.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HardDelete(c *gin.Context) {
	tenantID, id, ok := parseTenantAndID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	if err := h.uc.HardDelete(c.Request.Context(), tenantID, id, auth.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Void(c *gin.Context) {
	tenantID, id, ok := parseTenantAndID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.Void(c.Request.Context(), tenantID, id, auth.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ListCreditNotes(c *gin.Context) {
	tenantID, ok := parseTenant(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, err := h.uc.ListCreditNotes(c.Request.Context(), tenantID, nil, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	handlers.WriteOffsetListResponse(c, items, limit, len(items))
}

func (h *Handler) CreateCreditNote(c *gin.Context) {
	tenantID, ok := parseTenant(c)
	if !ok {
		return
	}
	var req returndto.CreateCreditNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handlers.WriteValidation(c, "invalid request body")
		return
	}
	partyID, err := uuid.Parse(strings.TrimSpace(req.PartyID))
	if err != nil {
		handlers.WriteValidation(c, "invalid party_id")
		return
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.CreateManualCreditNote(c.Request.Context(), CreateManualCreditNoteInput{
		TenantID: tenantID,
		PartyID:  partyID,
		Amount:   req.Amount,
		Actor:    auth.Actor,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) GetCreditNote(c *gin.Context) {
	tenantID, id, ok := parseTenantAndID(c)
	if !ok {
		return
	}
	out, err := h.uc.GetCreditNote(c.Request.Context(), tenantID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ListPartyCreditNotes(c *gin.Context) {
	tenantID, partyID, ok := parseTenantAndID(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, err := h.uc.ListCreditNotes(c.Request.Context(), tenantID, &partyID, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	handlers.WriteOffsetListResponse(c, items, limit, len(items))
}

func (h *Handler) ApplyCredit(c *gin.Context) {
	tenantID, saleID, ok := parseTenantSale(c)
	if !ok {
		return
	}
	var req applyCreditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handlers.WriteValidation(c, "invalid request body")
		return
	}
	creditNoteID, err := uuid.Parse(strings.TrimSpace(req.CreditNoteID))
	if err != nil {
		handlers.WriteValidation(c, "invalid credit_note_id")
		return
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.ApplyCredit(c.Request.Context(), ApplyCreditInput{TenantID: tenantID, SaleID: saleID, CreditNoteID: creditNoteID, Amount: req.Amount, Actor: auth.Actor})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func parseTenant(c *gin.Context) (uuid.UUID, bool) {
	return handlers.ParseAuthTenantID(c)
}

func parseTenantAndID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	return handlers.ParseAuthTenantAndParamID(c, "id", "id")
}

func parseTenantSale(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	tenantID, ok := parseTenant(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	saleID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		handlers.WriteValidation(c, "invalid sale id")
		return uuid.Nil, uuid.Nil, false
	}
	return tenantID, saleID, true
}
