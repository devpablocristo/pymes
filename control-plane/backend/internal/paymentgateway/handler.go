package paymentgateway

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/paymentgateway/handler/dto"
	gatewaydomain "github.com/devpablocristo/pymes/control-plane/backend/internal/paymentgateway/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
)

type gatewayUsecases interface {
	GetConnectionStatus(ctx context.Context, orgID uuid.UUID) (gatewaydomain.ConnectionStatus, error)
	InitOAuth(ctx context.Context, orgID uuid.UUID) (string, error)
	HandleOAuthCallback(ctx context.Context, state, code string) (uuid.UUID, error)
	Disconnect(ctx context.Context, orgID uuid.UUID) error

	CreatePreference(ctx context.Context, orgID uuid.UUID, req CreatePreferenceRequest) (gatewaydomain.PaymentPreference, error)
	GetPreference(ctx context.Context, orgID uuid.UUID, refType string, refID uuid.UUID) (gatewaydomain.PaymentPreference, error)
	GetOrCreatePreference(ctx context.Context, orgID uuid.UUID, req CreatePreferenceRequest) (gatewaydomain.PaymentPreference, error)
	GetPublicQuotePaymentLink(ctx context.Context, orgRef string, quoteID uuid.UUID) (gatewaydomain.PaymentPreference, error)

	GenerateStaticQR(ctx context.Context, orgID uuid.UUID, size int) ([]byte, error)
	BuildSalePaymentInfoWhatsApp(ctx context.Context, orgID uuid.UUID, saleID uuid.UUID) (WhatsAppResult, error)
	BuildSalePaymentLinkWhatsApp(ctx context.Context, orgID uuid.UUID, saleID uuid.UUID) (gatewaydomain.PaymentPreference, WhatsAppResult, error)

	ProcessWebhook(ctx context.Context, provider string, headers http.Header, body []byte) error
}

type Handler struct {
	uc gatewayUsecases
}

func NewHandler(uc gatewayUsecases) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) RegisterAuthRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/payment-gateway/connect", rbac.RequirePermission("billing", "read"), h.Connect)
	auth.GET("/payment-gateway/status", rbac.RequirePermission("billing", "read"), h.Status)
	auth.DELETE("/payment-gateway/disconnect", rbac.RequirePermission("billing", "read"), h.Disconnect)

	auth.GET("/payment-methods/qr-static", rbac.RequirePermission("sales", "read"), h.GetStaticQR)
	auth.GET("/payment-methods/qr-static/download", rbac.RequirePermission("sales", "read"), h.DownloadStaticQR)

	auth.POST("/sales/:id/payment-link", rbac.RequirePermission("sales", "create"), h.CreateSalePaymentLink)
	auth.GET("/sales/:id/payment-link", rbac.RequirePermission("sales", "read"), h.GetSalePaymentLink)
	auth.POST("/quotes/:id/payment-link", rbac.RequirePermission("quotes", "update"), h.CreateQuotePaymentLink)
	auth.GET("/quotes/:id/payment-link", rbac.RequirePermission("quotes", "read"), h.GetQuotePaymentLink)

	auth.GET("/whatsapp/sale/:id/payment-info", rbac.RequirePermission("sales", "read"), h.GetSalePaymentInfoWhatsApp)
	auth.GET("/whatsapp/sale/:id/payment-link", rbac.RequirePermission("sales", "read"), h.GetSalePaymentLinkWhatsApp)
}

func (h *Handler) RegisterPublicRoutes(v1 *gin.RouterGroup) {
	v1.GET("/payment-gateway/callback", h.Callback)
	v1.POST("/webhooks/mercadopago", h.MercadoPagoWebhook)
}

func (h *Handler) RegisterExternalRoutes(public *gin.RouterGroup) {
	public.GET("/quote/:id/payment-link", h.GetPublicQuotePaymentLink)
}

func (h *Handler) Connect(c *gin.Context) {
	orgID, ok := parseAuthOrgID(c)
	if !ok {
		return
	}
	redirectURL, err := h.uc.InitOAuth(c.Request.Context(), orgID)
	if err != nil {
		handleGatewayError(c, err)
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

func (h *Handler) Callback(c *gin.Context) {
	code := strings.TrimSpace(c.Query("code"))
	state := strings.TrimSpace(c.Query("state"))

	orgID, err := h.uc.HandleOAuthCallback(c.Request.Context(), state, code)
	if err != nil {
		handleGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":     true,
		"org_id": orgID.String(),
	})
}

func (h *Handler) Status(c *gin.Context) {
	orgID, ok := parseAuthOrgID(c)
	if !ok {
		return
	}
	status, err := h.uc.GetConnectionStatus(c.Request.Context(), orgID)
	if err != nil {
		handleGatewayError(c, err)
		return
	}

	resp := dto.ConnectionStatusResponse{
		Connected:      status.Connected,
		Provider:       status.Provider,
		ExternalUserID: status.ExternalUserID,
	}
	if status.TokenExpiresAt != nil {
		ts := status.TokenExpiresAt.UTC().Format(time.RFC3339)
		resp.TokenExpiresAt = &ts
	}
	if status.ConnectedAt != nil {
		ts := status.ConnectedAt.UTC().Format(time.RFC3339)
		resp.ConnectedAt = &ts
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Disconnect(c *gin.Context) {
	orgID, ok := parseAuthOrgID(c)
	if !ok {
		return
	}
	if err := h.uc.Disconnect(c.Request.Context(), orgID); err != nil {
		handleGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) GetStaticQR(c *gin.Context) {
	orgID, ok := parseAuthOrgID(c)
	if !ok {
		return
	}
	png, err := h.uc.GenerateStaticQR(c.Request.Context(), orgID, 512)
	if err != nil {
		handleGatewayError(c, err)
		return
	}
	c.Data(http.StatusOK, "image/png", png)
}

func (h *Handler) DownloadStaticQR(c *gin.Context) {
	orgID, ok := parseAuthOrgID(c)
	if !ok {
		return
	}
	png, err := h.uc.GenerateStaticQR(c.Request.Context(), orgID, 1024)
	if err != nil {
		handleGatewayError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="qr-static.png"`)
	c.Data(http.StatusOK, "image/png", png)
}

func (h *Handler) CreateSalePaymentLink(c *gin.Context) {
	orgID, ok := parseAuthOrgID(c)
	if !ok {
		return
	}
	saleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sale id"})
		return
	}

	pref, err := h.uc.CreatePreference(c.Request.Context(), orgID, CreatePreferenceRequest{
		ReferenceType: "sale",
		ReferenceID:   saleID,
	})
	if err != nil {
		handleGatewayError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toPaymentLinkResponse(pref))
}

func (h *Handler) GetSalePaymentLink(c *gin.Context) {
	orgID, ok := parseAuthOrgID(c)
	if !ok {
		return
	}
	saleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sale id"})
		return
	}

	pref, err := h.uc.GetPreference(c.Request.Context(), orgID, "sale", saleID)
	if err != nil {
		handleGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPaymentLinkResponse(pref))
}

func (h *Handler) CreateQuotePaymentLink(c *gin.Context) {
	orgID, ok := parseAuthOrgID(c)
	if !ok {
		return
	}
	quoteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quote id"})
		return
	}

	pref, err := h.uc.CreatePreference(c.Request.Context(), orgID, CreatePreferenceRequest{
		ReferenceType: "quote",
		ReferenceID:   quoteID,
	})
	if err != nil {
		handleGatewayError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toPaymentLinkResponse(pref))
}

func (h *Handler) GetQuotePaymentLink(c *gin.Context) {
	orgID, ok := parseAuthOrgID(c)
	if !ok {
		return
	}
	quoteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quote id"})
		return
	}

	pref, err := h.uc.GetPreference(c.Request.Context(), orgID, "quote", quoteID)
	if err != nil {
		handleGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPaymentLinkResponse(pref))
}

func (h *Handler) GetSalePaymentInfoWhatsApp(c *gin.Context) {
	orgID, ok := parseAuthOrgID(c)
	if !ok {
		return
	}
	saleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sale id"})
		return
	}

	wa, err := h.uc.BuildSalePaymentInfoWhatsApp(c.Request.Context(), orgID, saleID)
	if err != nil {
		handleGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.WhatsAppResponse{URL: wa.URL, Message: wa.Message})
}

func (h *Handler) GetSalePaymentLinkWhatsApp(c *gin.Context) {
	orgID, ok := parseAuthOrgID(c)
	if !ok {
		return
	}
	saleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sale id"})
		return
	}

	pref, wa, err := h.uc.BuildSalePaymentLinkWhatsApp(c.Request.Context(), orgID, saleID)
	if err != nil {
		handleGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"payment_link": toPaymentLinkResponse(pref),
		"whatsapp":     dto.WhatsAppResponse{URL: wa.URL, Message: wa.Message},
	})
}

func (h *Handler) GetPublicQuotePaymentLink(c *gin.Context) {
	quoteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quote id"})
		return
	}
	orgRef := strings.TrimSpace(c.Param("org_id"))
	pref, err := h.uc.GetPublicQuotePaymentLink(c.Request.Context(), orgRef, quoteID)
	if err != nil {
		handleGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPaymentLinkResponse(pref))
}

func (h *Handler) MercadoPagoWebhook(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if err := h.uc.ProcessWebhook(c.Request.Context(), providerMercadoPago, c.Request.Header, body); err != nil {
		handleGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func parseAuthOrgID(c *gin.Context) (uuid.UUID, bool) {
	auth := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(auth.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, false
	}
	return orgID, true
}

func toPaymentLinkResponse(in gatewaydomain.PaymentPreference) dto.PaymentLinkResponse {
	return dto.PaymentLinkResponse{
		ID:            in.ID.String(),
		Provider:      in.Provider,
		ReferenceType: in.ReferenceType,
		ReferenceID:   in.ReferenceID.String(),
		Status:        in.Status,
		Amount:        in.Amount,
		PaymentURL:    in.PaymentURL,
		QRData:        in.QRData,
		ExpiresAt:     in.ExpiresAt.UTC().Format(time.RFC3339),
		CreatedAt:     in.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func handleGatewayError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrPlanRestricted):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, ErrPlanMonthlyLimitReached):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
	case errors.Is(err, ErrGatewayNotConnected):
		c.JSON(http.StatusPreconditionFailed, gin.H{"error": "mercadopago no conectado"})
	case errors.Is(err, ErrInvalidOAuthState):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidWebhookSignature):
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
	case errors.Is(err, ErrBankAliasMissing):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case errors.Is(err, ErrUnsupportedProvider):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidReference):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
