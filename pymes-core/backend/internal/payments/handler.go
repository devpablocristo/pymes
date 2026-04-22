package payments

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	crudpaths "github.com/devpablocristo/modules/crud/paths/go/paths"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/payments/handler/dto"
	paymentsdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/payments/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	ListSalePayments(ctx context.Context, orgID, saleID uuid.UUID) ([]paymentsdomain.Payment, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]paymentsdomain.Payment, error)
	CreateSalePayment(ctx context.Context, orgID, saleID uuid.UUID, in paymentsdomain.Payment) (paymentsdomain.Payment, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (paymentsdomain.Payment, error)
	Update(ctx context.Context, in paymentsdomain.Payment, actor string) (paymentsdomain.Payment, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/sales/:id/payments", rbac.RequirePermission("payments", "read"), h.ListSalePayments)
	auth.POST("/sales/:id/payments", rbac.RequirePermission("payments", "create"), h.CreateSalePayment)

	// CRUD canónico sobre recursos payments standalone (para la vista "Pagos" del frontend).
	const base = "/payments"
	const item = base + "/:id"
	auth.GET(base, rbac.RequirePermission("payments", "read"), h.List)
	auth.GET(base+"/"+crudpaths.SegmentArchived, rbac.RequirePermission("payments", "read"), h.ListArchived)
	auth.GET(item, rbac.RequirePermission("payments", "read"), h.Get)
	auth.PATCH(item, rbac.RequirePermission("payments", "update"), h.Update)
	auth.DELETE(item, rbac.RequirePermission("payments", "delete"), h.Delete)
	auth.POST(item+"/"+crudpaths.SegmentArchive, rbac.RequirePermission("payments", "update"), h.Delete)
	auth.POST(item+"/"+crudpaths.SegmentRestore, rbac.RequirePermission("payments", "update"), h.RestoreAction)
	auth.DELETE(item+"/"+crudpaths.SegmentHard, rbac.RequirePermission("payments", "delete"), h.HardDelete)
}

func (h *Handler) ListSalePayments(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, saleID, ok := parseOrgSale(c, authCtx.OrgID)
	if !ok {
		return
	}
	items, err := h.uc.ListSalePayments(c.Request.Context(), orgID, saleID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) CreateSalePayment(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, saleID, ok := parseOrgSale(c, authCtx.OrgID)
	if !ok {
		return
	}
	var req dto.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	receivedAt := time.Now().UTC()
	if strings.TrimSpace(req.ReceivedAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ReceivedAt))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid received_at"})
			return
		}
		receivedAt = parsed.UTC()
	}
	isFavorite := false
	if req.IsFavorite != nil {
		isFavorite = *req.IsFavorite
	}
	out, err := h.uc.CreateSalePayment(c.Request.Context(), orgID, saleID, paymentsdomain.Payment{Method: req.Method, Amount: req.Amount, Notes: strings.TrimSpace(req.Notes), ReceivedAt: receivedAt, IsFavorite: isFavorite, Tags: req.Tags, CreatedBy: authCtx.Actor})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toPaymentItem(out))
}

// List devuelve pagos scoped por saleID vía ?sale_id=; no hay listado global.
func (h *Handler) List(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	saleIDRaw := strings.TrimSpace(c.Query("sale_id"))
	if saleIDRaw == "" {
		// Sin sale_id no hay listado global: devolvemos lista vacía para que el CRUD del frontend
		// se renderice sin error.
		c.JSON(http.StatusOK, gin.H{"items": []any{}})
		return
	}
	saleID, err := uuid.Parse(saleIDRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sale_id"})
		return
	}
	items, err := h.uc.ListSalePayments(c.Request.Context(), orgID, saleID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := make([]dto.PaymentItem, 0, len(items))
	for _, it := range items {
		resp = append(resp, toPaymentItem(it))
	}
	c.JSON(http.StatusOK, gin.H{"items": resp})
}

func (h *Handler) ListArchived(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
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
	resp := make([]dto.PaymentItem, 0, len(items))
	for _, it := range items {
		resp = append(resp, toPaymentItem(it))
	}
	c.JSON(http.StatusOK, gin.H{"items": resp})
}

func (h *Handler) Get(c *gin.Context) {
	orgID, id, ok := parsePaymentOrgID(c)
	if !ok {
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toPaymentItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, id, ok := parsePaymentOrgID(c)
	if !ok {
		return
	}
	var req dto.UpdatePaymentRequest
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
	out, err := h.uc.Update(c.Request.Context(), current, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toPaymentItem(out))
}

// Delete realiza soft delete (archiva).
func (h *Handler) Delete(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, id, ok := parsePaymentOrgID(c)
	if !ok {
		return
	}
	if err := h.uc.SoftDelete(c.Request.Context(), orgID, id, authCtx.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) RestoreAction(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, id, ok := parsePaymentOrgID(c)
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
	orgID, id, ok := parsePaymentOrgID(c)
	if !ok {
		return
	}
	if err := h.uc.HardDelete(c.Request.Context(), orgID, id, authCtx.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func parsePaymentOrgID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
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

func toPaymentItem(in paymentsdomain.Payment) dto.PaymentItem {
	out := dto.PaymentItem{
		ID:            in.ID.String(),
		OrgID:         in.OrgID.String(),
		ReferenceType: in.ReferenceType,
		ReferenceID:   in.ReferenceID.String(),
		Method:        in.Method,
		Amount:        in.Amount,
		Notes:         in.Notes,
		ReceivedAt:    in.ReceivedAt.UTC().Format(time.RFC3339),
		IsFavorite:    in.IsFavorite,
		Tags:          append([]string(nil), in.Tags...),
		CreatedBy:     in.CreatedBy,
		CreatedAt:     in.CreatedAt.UTC().Format(time.RFC3339),
	}
	if in.ArchivedAt != nil {
		out.ArchivedAt = in.ArchivedAt.UTC().Format(time.RFC3339)
	}
	return out
}

func parseOrgSale(c *gin.Context, rawOrgID string) (uuid.UUID, uuid.UUID, bool) {
	orgID, err := uuid.Parse(rawOrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, uuid.Nil, false
	}
	saleID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sale id"})
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, saleID, true
}
