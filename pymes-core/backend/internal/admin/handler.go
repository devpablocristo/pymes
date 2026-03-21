package admin

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/admin/handler/dto"
	admindomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/admin/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/authz"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	GetBootstrap(ctx context.Context, orgID string, role string, scopes []string, actor string, authMethod string) (map[string]any, error)
	GetTenantSettings(ctx context.Context, orgID string) (admindomain.TenantSettings, error)
	UpdateTenantSettings(ctx context.Context, orgID string, patch admindomain.TenantSettingsPatch, actor *string) (admindomain.TenantSettings, error)
	ListActivity(ctx context.Context, orgID string, limit int) ([]admindomain.ActivityEvent, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup) {
	// GET /session vive en wire/saas_http.go (mux SaaS) con envelope de producto; ver core/saas/go/session.
	auth.GET("/admin/bootstrap", h.GetBootstrap)
	auth.GET("/admin/tenant-settings", h.GetTenantSettings)
	auth.PUT("/admin/tenant-settings", h.UpdateTenantSettings)
	auth.PATCH("/admin/tenant-settings", h.UpdateTenantSettings)
	auth.GET("/tenant-settings", h.GetTenantSettings)
	auth.PATCH("/tenant-settings", h.UpdateTenantSettings)
	auth.GET("/admin/activity", h.ListActivity)
}

func (h *Handler) GetBootstrap(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	if !authz.IsAdmin(authCtx.Role, authCtx.Scopes) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin permissions required"})
		return
	}
	payload, err := h.uc.GetBootstrap(c.Request.Context(), authCtx.OrgID, authCtx.Role, authCtx.Scopes, authCtx.Actor, authCtx.AuthMethod)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, payload)
}

func (h *Handler) GetTenantSettings(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	if !authz.CanReadConsoleSettings(authCtx.Role, authCtx.Scopes) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin read permission required"})
		return
	}
	settings, err := h.uc.GetTenantSettings(c.Request.Context(), authCtx.OrgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *Handler) UpdateTenantSettings(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	if !authz.CanWriteConsoleSettings(authCtx.Role, authCtx.Scopes) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin write permission required"})
		return
	}
	var req dto.UpdateTenantSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	updated, err := h.uc.UpdateTenantSettings(c.Request.Context(), authCtx.OrgID, admindomain.TenantSettingsPatch{
		PlanCode:                 req.PlanCode,
		HardLimits:               req.HardLimits,
		Currency:                 req.Currency,
		TaxRate:                  req.TaxRate,
		QuotePrefix:              req.QuotePrefix,
		SalePrefix:               req.SalePrefix,
		AllowNegativeStock:       req.AllowNegativeStock,
		PurchasePrefix:           req.PurchasePrefix,
		ReturnPrefix:             req.ReturnPrefix,
		CreditNotePrefix:         req.CreditNotePrefix,
		BusinessName:             req.BusinessName,
		BusinessTaxID:            req.BusinessTaxID,
		BusinessAddress:          req.BusinessAddress,
		BusinessPhone:            req.BusinessPhone,
		BusinessEmail:            req.BusinessEmail,
		WAQuoteTemplate:          req.WAQuoteTemplate,
		WAReceiptTemplate:        req.WAReceiptTemplate,
		WADefaultCountryCode:     req.WADefaultCountryCode,
		AppointmentsEnabled:      req.AppointmentsEnabled,
		AppointmentLabel:         req.AppointmentLabel,
		AppointmentReminderHours: req.AppointmentReminderHours,
		SecondaryCurrency:        req.SecondaryCurrency,
		DefaultRateType:          req.DefaultRateType,
		AutoFetchRates:           req.AutoFetchRates,
		ShowDualPrices:           req.ShowDualPrices,
		BankHolder:               req.BankHolder,
		BankCBU:                  req.BankCBU,
		BankAlias:                req.BankAlias,
		BankName:                 req.BankName,
		ShowQRInPDF:              req.ShowQRInPDF,
		WAPaymentTemplate:        req.WAPaymentTemplate,
		WAPaymentLinkTemplate:    req.WAPaymentLinkTemplate,
	}, &authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) ListActivity(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	if !authz.CanReadConsoleSettings(authCtx.Role, authCtx.Scopes) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin read permission required"})
		return
	}
	items, err := h.uc.ListActivity(c.Request.Context(), authCtx.OrgID, 200)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
