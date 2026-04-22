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
	List(ctx context.Context, orgID uuid.UUID, limit int) ([]returndomain.Return, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]returndomain.Return, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (returndomain.Return, error)
	Create(ctx context.Context, in CreateReturnInput) (returndomain.Return, *returndomain.CreditNote, error)
	Update(ctx context.Context, in returndomain.Return, actor string) (returndomain.Return, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
	RestoreArchived(ctx context.Context, orgID, id uuid.UUID, actor string) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Void(ctx context.Context, orgID, id uuid.UUID, actor string) (returndomain.Return, error)
	ListCreditNotes(ctx context.Context, orgID uuid.UUID, partyID *uuid.UUID, limit int) ([]returndomain.CreditNote, error)
	GetCreditNote(ctx context.Context, orgID, id uuid.UUID) (returndomain.CreditNote, error)
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
	auth.POST(item+"/"+crudpaths.SegmentArchive, rbac.RequirePermission("returns", "update"), h.Delete)
	auth.POST(item+"/"+crudpaths.SegmentRestore, rbac.RequirePermission("returns", "update"), h.RestoreAction)
	auth.DELETE(item+"/"+crudpaths.SegmentHard, rbac.RequirePermission("returns", "delete"), h.HardDelete)

	auth.GET("/credit-notes", rbac.RequirePermission("returns", "read"), h.ListCreditNotes)
	auth.POST("/credit-notes", rbac.RequirePermission("returns", "create"), h.CreateCreditNote)
	auth.GET("/credit-notes/:id", rbac.RequirePermission("returns", "read"), h.GetCreditNote)
	auth.GET("/parties/:id/credit-notes", rbac.RequirePermission("returns", "read"), h.ListPartyCreditNotes)
	auth.POST("/sales/:id/apply-credit", rbac.RequirePermission("payments", "create"), h.ApplyCredit)
}

func (h *Handler) Create(c *gin.Context) {
	orgID, saleID, ok := parseOrgSale(c)
	if !ok {
		return
	}
	var req createReturnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	auth := handlers.GetAuthContext(c)
	items := make([]CreateReturnItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		saleItemID, err := uuid.Parse(strings.TrimSpace(item.SaleItemID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sale_item_id"})
			return
		}
		items = append(items, CreateReturnItemInput{SaleItemID: saleItemID, Quantity: item.Quantity})
	}
	out, credit, err := h.uc.Create(c.Request.Context(), CreateReturnInput{OrgID: orgID, SaleID: saleID, Reason: req.Reason, RefundMethod: strings.TrimSpace(req.RefundMethod), Notes: strings.TrimSpace(req.Notes), CreatedBy: auth.Actor, Items: items})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"return": out, "credit_note": credit})
}

func (h *Handler) List(c *gin.Context) {
	orgID, ok := parseOrg(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, err := h.uc.List(c.Request.Context(), orgID, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
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

func (h *Handler) ListArchived(c *gin.Context) {
	orgID, ok := parseOrg(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, err := h.uc.ListArchived(c.Request.Context(), orgID, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) Update(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	var req updateReturnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	current, err := h.uc.GetByID(c.Request.Context(), orgID, id)
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
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	if err := h.uc.SoftDelete(c.Request.Context(), orgID, id, auth.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) RestoreAction(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	if err := h.uc.RestoreArchived(c.Request.Context(), orgID, id, auth.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HardDelete(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	if err := h.uc.HardDelete(c.Request.Context(), orgID, id, auth.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Void(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.Void(c.Request.Context(), orgID, id, auth.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ListCreditNotes(c *gin.Context) {
	orgID, ok := parseOrg(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, err := h.uc.ListCreditNotes(c.Request.Context(), orgID, nil, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) CreateCreditNote(c *gin.Context) {
	orgID, ok := parseOrg(c)
	if !ok {
		return
	}
	var req returndto.CreateCreditNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	partyID, err := uuid.Parse(strings.TrimSpace(req.PartyID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid party_id"})
		return
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.CreateManualCreditNote(c.Request.Context(), CreateManualCreditNoteInput{
		OrgID:   orgID,
		PartyID: partyID,
		Amount:  req.Amount,
		Actor:   auth.Actor,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) GetCreditNote(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	out, err := h.uc.GetCreditNote(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ListPartyCreditNotes(c *gin.Context) {
	orgID, partyID, ok := parseOrgID(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, err := h.uc.ListCreditNotes(c.Request.Context(), orgID, &partyID, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) ApplyCredit(c *gin.Context) {
	orgID, saleID, ok := parseOrgSale(c)
	if !ok {
		return
	}
	var req applyCreditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	creditNoteID, err := uuid.Parse(strings.TrimSpace(req.CreditNoteID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credit_note_id"})
		return
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.ApplyCredit(c.Request.Context(), ApplyCreditInput{OrgID: orgID, SaleID: saleID, CreditNoteID: creditNoteID, Amount: req.Amount, Actor: auth.Actor})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func parseOrg(c *gin.Context) (uuid.UUID, bool) {
	return handlers.ParseAuthOrgID(c)
}

func parseOrgID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	return handlers.ParseAuthOrgAndParamID(c, "id", "id")
}

func parseOrgSale(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	orgID, ok := parseOrg(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	saleID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sale id"})
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, saleID, true
}
