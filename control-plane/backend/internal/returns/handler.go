package returns

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	returndomain "github.com/devpablocristo/pymes/control-plane/backend/internal/returns/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pkgs/go-pkg/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, orgID uuid.UUID, limit int) ([]returndomain.Return, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (returndomain.Return, error)
	Create(ctx context.Context, in CreateReturnInput) (returndomain.Return, *returndomain.CreditNote, error)
	Void(ctx context.Context, orgID, id uuid.UUID, actor string) (returndomain.Return, error)
	ListCreditNotes(ctx context.Context, orgID uuid.UUID, partyID *uuid.UUID, limit int) ([]returndomain.CreditNote, error)
	GetCreditNote(ctx context.Context, orgID, id uuid.UUID) (returndomain.CreditNote, error)
	ApplyCredit(ctx context.Context, in ApplyCreditInput) (returndomain.CreditNote, error)
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.POST("/sales/:id/return", rbac.RequirePermission("returns", "create"), h.Create)
	auth.GET("/returns", rbac.RequirePermission("returns", "read"), h.List)
	auth.GET("/returns/:id", rbac.RequirePermission("returns", "read"), h.Get)
	auth.POST("/returns/:id/void", rbac.RequirePermission("returns", "create"), h.Void)
	auth.GET("/credit-notes", rbac.RequirePermission("returns", "read"), h.ListCreditNotes)
	auth.GET("/credit-notes/:id", rbac.RequirePermission("returns", "read"), h.GetCreditNote)
	auth.GET("/parties/:id/credit-notes", rbac.RequirePermission("returns", "read"), h.ListPartyCreditNotes)
	auth.POST("/sales/:id/apply-credit", rbac.RequirePermission("payments", "create"), h.ApplyCredit)
}

func (h *Handler) Create(c *gin.Context) {
	orgID, saleID, ok := parseOrgSale(c)
	if !ok {
		return
	}
	var req struct {
		Reason       string `json:"reason"`
		RefundMethod string `json:"refund_method" binding:"required"`
		Notes        string `json:"notes"`
		Items        []struct {
			SaleItemID string  `json:"sale_item_id" binding:"required"`
			Quantity   float64 `json:"quantity" binding:"required"`
		} `json:"items" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
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
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, err := h.uc.ListCreditNotes(c.Request.Context(), orgID, nil, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
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
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
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
	var req struct {
		CreditNoteID string  `json:"credit_note_id" binding:"required"`
		Amount       float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
	auth := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(auth.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, false
	}
	return orgID, true
}

func parseOrgID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	orgID, ok := parseOrg(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, id, true
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
