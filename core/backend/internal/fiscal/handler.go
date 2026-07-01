package fiscal

import (
	"context"
	"net/http"
	"strings"

	"github.com/devpablocristo/platform/http/go/pagination"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/core/backend/internal/fiscal/handler/dto"
	fiscaldomain "github.com/devpablocristo/pymes/core/backend/internal/fiscal/usecases/domain"
	"github.com/devpablocristo/pymes/core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/core/shared/backend/httperrors"
)

type usecasesPort interface {
	GetSettings(ctx context.Context, orgID uuid.UUID) (fiscaldomain.FiscalSettings, error)
	SaveSettings(ctx context.Context, orgID uuid.UUID, in SaveSettingsInput) (fiscaldomain.FiscalSettings, error)
	Authenticate(ctx context.Context, orgID uuid.UUID) (fiscaldomain.AuthTicket, error)
	EmitVoucher(ctx context.Context, orgID uuid.UUID, in EmitInput) (fiscaldomain.FiscalVoucher, error)
	EmitCreditNote(ctx context.Context, orgID uuid.UUID, in EmitCreditNoteInput) (fiscaldomain.FiscalVoucher, error)
	GetVoucher(ctx context.Context, orgID, id uuid.UUID) (fiscaldomain.FiscalVoucher, error)
	ListVouchers(ctx context.Context, orgID uuid.UUID, limit int) ([]fiscaldomain.FiscalVoucher, error)
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	const base = "/fiscal"
	auth.GET(base+"/settings", rbac.RequirePermission("fiscal", "read"), h.GetSettings)
	auth.PUT(base+"/settings", rbac.RequirePermission("fiscal", "update"), h.SaveSettings)
	auth.POST(base+"/test-auth", rbac.RequirePermission("fiscal", "update"), h.TestAuth)
	auth.POST(base+"/vouchers", rbac.RequirePermission("fiscal", "create"), h.EmitVoucher)
	auth.POST(base+"/credit-notes", rbac.RequirePermission("fiscal", "create"), h.EmitCreditNote)
	auth.GET(base+"/vouchers", rbac.RequirePermission("fiscal", "read"), h.ListVouchers)
	auth.GET(base+"/vouchers/:id", rbac.RequirePermission("fiscal", "read"), h.GetVoucher)
}

func (h *Handler) GetSettings(c *gin.Context) {
	orgID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	out, err := h.uc.GetSettings(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) SaveSettings(c *gin.Context) {
	orgID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	var req dto.SaveSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handlers.WriteValidation(c, "invalid request body")
		return
	}
	out, err := h.uc.SaveSettings(c.Request.Context(), orgID, SaveSettingsInput{
		CUIT:               req.CUIT,
		Environment:        req.Environment,
		TaxCondition:       req.TaxCondition,
		DefaultPointOfSale: req.DefaultPointOfSale,
		Enabled:            req.Enabled,
		CertPEM:            req.CertPEM,
		KeyPEM:             req.KeyPEM,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) EmitVoucher(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil {
		handlers.WriteValidation(c, "invalid tenant")
		return
	}
	var req dto.EmitVoucherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handlers.WriteValidation(c, "invalid request body")
		return
	}
	saleID, err := uuid.Parse(strings.TrimSpace(req.SaleID))
	if err != nil {
		handlers.WriteValidation(c, "invalid sale_id")
		return
	}
	out, err := h.uc.EmitVoucher(c.Request.Context(), orgID, EmitInput{
		SaleID: saleID, VoucherType: req.VoucherType, PointOfSale: req.PointOfSale,
		Concepto: req.Concepto, ServiceFrom: req.ServiceFrom, ServiceTo: req.ServiceTo,
		PaymentDue: req.PaymentDue, ExchangeRate: req.ExchangeRate, Actor: authCtx.Actor,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) EmitCreditNote(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil {
		handlers.WriteValidation(c, "invalid tenant")
		return
	}
	var req dto.EmitCreditNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handlers.WriteValidation(c, "invalid request body")
		return
	}
	returnID, err := uuid.Parse(strings.TrimSpace(req.ReturnID))
	if err != nil {
		handlers.WriteValidation(c, "invalid return_id")
		return
	}
	out, err := h.uc.EmitCreditNote(c.Request.Context(), orgID, EmitCreditNoteInput{
		ReturnID: returnID, PointOfSale: req.PointOfSale, Actor: authCtx.Actor,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) GetVoucher(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	out, err := h.uc.GetVoucher(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ListVouchers(c *gin.Context) {
	orgID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "50", pagination.Config{DefaultLimit: 50, MaxLimit: 200})
	items, err := h.uc.ListVouchers(c.Request.Context(), orgID, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// TestAuth valida la configuración autenticando contra el WSAA de ARCA.
func (h *Handler) TestAuth(c *gin.Context) {
	orgID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	ta, err := h.uc.Authenticate(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"authenticated": true, "expires_at": ta.ExpiresAt})
}
