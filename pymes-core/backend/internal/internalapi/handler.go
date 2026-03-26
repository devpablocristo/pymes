// Package internalapi exposes internal service-to-service routes for the pymes-core.
package internalapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	admindomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/admin/usecases/domain"
	appointmentsdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/appointments/usecases/domain"
	customerdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/customers/usecases/domain"
	partydomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/party/usecases/domain"
	gatewaydomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/paymentgateway/usecases/domain"
	productdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/products/usecases/domain"
	quotedomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/quotes/usecases/domain"
	saledomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/sales/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customers"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/paymentgateway"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/products"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/quotes"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/sales"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/users"
	wapdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/whatsapp/usecases/domain"
)

type adminPort interface {
	GetBootstrap(ctx context.Context, orgID string, role string, scopes []string, actor string, authMethod string) (map[string]any, error)
	GetTenantSettings(ctx context.Context, orgID string) (admindomain.TenantSettings, error)
}

type partyPort interface {
	GetByID(ctx context.Context, orgID, id uuid.UUID) (partydomain.Party, error)
}

type customerPort interface {
	List(ctx context.Context, p customers.ListParams) ([]customerdomain.Customer, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in customerdomain.Customer, actor string) (customerdomain.Customer, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (customerdomain.Customer, error)
}

type productPort interface {
	List(ctx context.Context, p products.ListParams) ([]productdomain.Product, int64, bool, *uuid.UUID, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (productdomain.Product, error)
}

type appointmentPort interface {
	List(ctx context.Context, orgID uuid.UUID, from, to *time.Time, status, assigned string, limit int, archived bool) ([]appointmentsdomain.Appointment, error)
	Create(ctx context.Context, in appointmentsdomain.Appointment) (appointmentsdomain.Appointment, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (appointmentsdomain.Appointment, error)
}

type quotePort interface {
	Create(ctx context.Context, in quotes.CreateQuoteInput) (quotedomain.Quote, error)
}

type salePort interface {
	Create(ctx context.Context, in sales.CreateSaleInput) (saledomain.Sale, error)
}

type paymentGatewayPort interface {
	GetOrCreatePreference(ctx context.Context, orgID uuid.UUID, req paymentgateway.CreatePreferenceRequest) (gatewaydomain.PaymentPreference, error)
}

type apiKeyResolverPort interface {
	ResolveAPIKey(raw string) (users.ResolvedAPIKey, bool)
}

type whatsAppSendPort interface {
	SendText(ctx context.Context, req wapdomain.SendTextRequest) (wapdomain.Message, error)
}

type Handler struct {
	admin     adminPort
	parties   partyPort
	customers customerPort
	products  productPort
	appts     appointmentPort
	quotes    quotePort
	sales     salePort
	gateway   paymentGatewayPort
	apiKeys   apiKeyResolverPort
	whatsapp  whatsAppSendPort
	// resolveOrgRef traduce Clerk org_... / slug / UUID (opcional; nil = ruta no registrada).
	resolveOrgRef func(context.Context, string) (uuid.UUID, bool, error)
}

func NewHandler(
	admin adminPort,
	parties partyPort,
	customers customerPort,
	products productPort,
	appts appointmentPort,
	quotes quotePort,
	sales salePort,
	gateway paymentGatewayPort,
	apiKeys apiKeyResolverPort,
	whatsapp whatsAppSendPort,
	resolveOrgRef func(context.Context, string) (uuid.UUID, bool, error),
) *Handler {
	return &Handler{
		admin:         admin,
		parties:       parties,
		customers:     customers,
		products:      products,
		appts:         appts,
		quotes:        quotes,
		sales:         sales,
		gateway:       gateway,
		apiKeys:       apiKeys,
		whatsapp:      whatsapp,
		resolveOrgRef: resolveOrgRef,
	}
}

func (h *Handler) RegisterRoutes(internal *gin.RouterGroup) {
	internal.GET("/orgs/:org_id/bootstrap", h.GetBootstrap)
	internal.GET("/orgs/:org_id/settings", h.GetSettings)
	if h.resolveOrgRef != nil {
		internal.GET("/orgs/resolve-ref", h.ResolveOrgRef)
	}
	internal.POST("/api-keys/resolve", h.ResolveAPIKey)
	internal.GET("/parties/:party_id", h.GetParty)
	internal.GET("/customers/:id", h.GetCustomer)
	internal.POST("/customers/resolve", h.ResolveCustomer)
	internal.GET("/products", h.ListProducts)
	internal.GET("/products/:id", h.GetProduct)
	internal.POST("/appointments", h.CreateAppointment)
	internal.GET("/appointments/:id", h.GetAppointment)
	internal.POST("/quotes", h.CreateQuote)
	internal.POST("/sales", h.CreateSale)
	internal.POST("/sales/:id/payment-link", h.CreateSalePaymentLink)
	internal.POST("/whatsapp/send-text", h.InternalSendWhatsAppText)
}

func (h *Handler) GetBootstrap(c *gin.Context) {
	orgID := strings.TrimSpace(c.Param("org_id"))
	if orgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "org_id required"})
		return
	}
	result, err := h.admin.GetBootstrap(c.Request.Context(), orgID, "service", nil, "internal-service", "service_token")
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetSettings(c *gin.Context) {
	orgID := strings.TrimSpace(c.Param("org_id"))
	if orgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "org_id required"})
		return
	}
	result, err := h.admin.GetTenantSettings(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) ResolveOrgRef(c *gin.Context) {
	if h.resolveOrgRef == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "org resolve unavailable"})
		return
	}
	ref := strings.TrimSpace(c.Query("ref"))
	if ref == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ref query param required"})
		return
	}
	id, ok, err := h.resolveOrgRef(c.Request.Context(), ref)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve organization"})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"org_id": id.String()})
}

func (h *Handler) ResolveAPIKey(c *gin.Context) {
	if h.apiKeys == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "api key resolver unavailable"})
		return
	}
	var req struct {
		APIKey string `json:"api_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	key, ok := h.apiKeys.ResolveAPIKey(strings.TrimSpace(req.APIKey))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "api key not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":     key.ID.String(),
		"org_id": key.OrgID.String(),
		"scopes": key.Scopes,
	})
}

func (h *Handler) GetParty(c *gin.Context) {
	partyID, err := uuid.Parse(strings.TrimSpace(c.Param("party_id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid party_id"})
		return
	}
	orgID, err := uuid.Parse(strings.TrimSpace(c.GetHeader("X-Org-ID")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header required"})
		return
	}
	result, err := h.parties.GetByID(c.Request.Context(), orgID, partyID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) ResolveCustomer(c *gin.Context) {
	var req struct {
		OrgID string `json:"org_id" binding:"required"`
		Name  string `json:"name" binding:"required"`
		Phone string `json:"phone"`
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	orgID, err := uuid.Parse(req.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org_id"})
		return
	}
	results, _, _, _, err := h.customers.List(c.Request.Context(), customers.ListParams{
		OrgID:  orgID,
		Search: strings.TrimSpace(req.Name),
		Limit:  1,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	if len(results) > 0 {
		c.JSON(http.StatusOK, results[0])
		return
	}
	created, err := h.customers.Create(c.Request.Context(), customerdomain.Customer{
		OrgID: orgID,
		Name:  strings.TrimSpace(req.Name),
		Phone: strings.TrimSpace(req.Phone),
		Email: strings.TrimSpace(req.Email),
	}, "internal-service")
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) GetCustomer(c *gin.Context) {
	orgID, err := uuid.Parse(strings.TrimSpace(c.GetHeader("X-Org-ID")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header required"})
		return
	}
	customerID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	item, err := h.customers.GetByID(c.Request.Context(), orgID, customerID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":     item.ID.String(),
		"org_id": item.OrgID.String(),
		"type":   item.Type,
		"name":   item.Name,
		"tax_id": item.TaxID,
		"email":  item.Email,
		"phone":  item.Phone,
		"address": gin.H{
			"street":   item.Address.Street,
			"city":     item.Address.City,
			"state":    item.Address.State,
			"zip_code": item.Address.ZipCode,
			"country":  item.Address.Country,
		},
		"notes":      item.Notes,
		"tags":       item.Tags,
		"metadata":   item.Metadata,
		"created_at": item.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": item.UpdatedAt.UTC().Format(time.RFC3339),
	})
}

func (h *Handler) ListProducts(c *gin.Context) {
	orgID, err := uuid.Parse(strings.TrimSpace(c.GetHeader("X-Org-ID")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header required"})
		return
	}
	query := c.Query("q")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, total, hasMore, next, err := h.products.List(c.Request.Context(), products.ListParams{
		OrgID:  orgID,
		Search: query,
		Limit:  limit,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	nextCursor := ""
	if next != nil {
		nextCursor = next.String()
	}
	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"total":       total,
		"has_more":    hasMore,
		"next_cursor": nextCursor,
	})
}

func (h *Handler) GetProduct(c *gin.Context) {
	orgID, err := uuid.Parse(strings.TrimSpace(c.GetHeader("X-Org-ID")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header required"})
		return
	}
	productID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	item, err := h.products.GetByID(c.Request.Context(), orgID, productID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":          item.ID.String(),
		"org_id":      item.OrgID.String(),
		"type":        item.Type,
		"sku":         item.SKU,
		"name":        item.Name,
		"description": item.Description,
		"unit":        item.Unit,
		"price":       item.Price,
		"cost_price":  item.CostPrice,
		"tax_rate":    item.TaxRate,
		"track_stock": item.TrackStock,
		"tags":        item.Tags,
		"metadata":    item.Metadata,
		"created_at":  item.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":  item.UpdatedAt.UTC().Format(time.RFC3339),
	})
}

func (h *Handler) CreateAppointment(c *gin.Context) {
	var req struct {
		OrgID         string         `json:"org_id" binding:"required"`
		CustomerID    *string        `json:"customer_id,omitempty"`
		CustomerName  string         `json:"customer_name" binding:"required"`
		CustomerPhone string         `json:"customer_phone"`
		Title         string         `json:"title" binding:"required"`
		Description   string         `json:"description"`
		Status        string         `json:"status"`
		StartAt       string         `json:"start_at" binding:"required"`
		EndAt         string         `json:"end_at,omitempty"`
		Duration      int            `json:"duration,omitempty"`
		Location      string         `json:"location"`
		AssignedTo    string         `json:"assigned_to"`
		Notes         string         `json:"notes"`
		Metadata      map[string]any `json:"metadata,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	orgID, err := uuid.Parse(req.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org_id"})
		return
	}
	startAt, err := time.Parse(time.RFC3339, strings.TrimSpace(req.StartAt))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_at"})
		return
	}
	var endAt time.Time
	if strings.TrimSpace(req.EndAt) != "" {
		endAt, err = time.Parse(time.RFC3339, strings.TrimSpace(req.EndAt))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_at"})
			return
		}
	}
	var customerID *uuid.UUID
	if req.CustomerID != nil && strings.TrimSpace(*req.CustomerID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.CustomerID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer_id"})
			return
		}
		customerID = &id
	}
	out, err := h.appts.Create(c.Request.Context(), appointmentsdomain.Appointment{
		OrgID:         orgID,
		CustomerID:    customerID,
		CustomerName:  strings.TrimSpace(req.CustomerName),
		CustomerPhone: strings.TrimSpace(req.CustomerPhone),
		Title:         strings.TrimSpace(req.Title),
		Description:   strings.TrimSpace(req.Description),
		Status:        req.Status,
		StartAt:       startAt.UTC(),
		EndAt:         endAt.UTC(),
		Duration:      req.Duration,
		Location:      strings.TrimSpace(req.Location),
		AssignedTo:    strings.TrimSpace(req.AssignedTo),
		Notes:         strings.TrimSpace(req.Notes),
		Metadata:      req.Metadata,
		CreatedBy:     "internal-service",
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) GetAppointment(c *gin.Context) {
	orgID, err := uuid.Parse(strings.TrimSpace(c.GetHeader("X-Org-ID")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header required"})
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	out, err := h.appts.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) CreateQuote(c *gin.Context) {
	var req struct {
		OrgID        string `json:"org_id" binding:"required"`
		CustomerID   string `json:"customer_id"`
		CustomerName string `json:"customer_name"`
		Items        []struct {
			ProductID   string   `json:"product_id"`
			Description string   `json:"description"`
			Quantity    float64  `json:"quantity"`
			UnitPrice   float64  `json:"unit_price"`
			TaxRate     *float64 `json:"tax_rate,omitempty"`
		} `json:"items" binding:"required"`
		Notes      string  `json:"notes"`
		ValidUntil *string `json:"valid_until,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	orgID, err := uuid.Parse(req.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org_id"})
		return
	}
	var customerID *uuid.UUID
	if strings.TrimSpace(req.CustomerID) != "" {
		id, err := uuid.Parse(req.CustomerID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer_id"})
			return
		}
		customerID = &id
	}
	items := make([]quotes.QuoteItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		var productID *uuid.UUID
		if strings.TrimSpace(item.ProductID) != "" {
			id, err := uuid.Parse(item.ProductID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id in items"})
				return
			}
			productID = &id
		}
		items = append(items, quotes.QuoteItemInput{
			ProductID:   productID,
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			TaxRate:     item.TaxRate,
		})
	}
	var validUntil *time.Time
	if req.ValidUntil != nil && strings.TrimSpace(*req.ValidUntil) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.ValidUntil))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid valid_until"})
			return
		}
		validUntil = &t
	}
	out, err := h.quotes.Create(c.Request.Context(), quotes.CreateQuoteInput{
		OrgID:        orgID,
		CustomerID:   customerID,
		CustomerName: strings.TrimSpace(req.CustomerName),
		Items:        items,
		Notes:        strings.TrimSpace(req.Notes),
		ValidUntil:   validUntil,
		CreatedBy:    "internal-service",
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) CreateSale(c *gin.Context) {
	var req struct {
		OrgID         string `json:"org_id" binding:"required"`
		CustomerID    string `json:"customer_id"`
		CustomerName  string `json:"customer_name"`
		QuoteID       string `json:"quote_id"`
		PaymentMethod string `json:"payment_method"`
		Items         []struct {
			ProductID   string   `json:"product_id"`
			Description string   `json:"description"`
			Quantity    float64  `json:"quantity"`
			UnitPrice   float64  `json:"unit_price"`
			TaxRate     *float64 `json:"tax_rate,omitempty"`
		} `json:"items" binding:"required"`
		Notes string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	orgID, err := uuid.Parse(req.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org_id"})
		return
	}
	var customerID *uuid.UUID
	if strings.TrimSpace(req.CustomerID) != "" {
		id, err := uuid.Parse(req.CustomerID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer_id"})
			return
		}
		customerID = &id
	}
	var quoteID *uuid.UUID
	if strings.TrimSpace(req.QuoteID) != "" {
		id, err := uuid.Parse(req.QuoteID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quote_id"})
			return
		}
		quoteID = &id
	}
	items := make([]sales.CreateSaleItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		var productID *uuid.UUID
		if strings.TrimSpace(item.ProductID) != "" {
			id, err := uuid.Parse(item.ProductID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id in items"})
				return
			}
			productID = &id
		}
		items = append(items, sales.CreateSaleItemInput{
			ProductID:   productID,
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			TaxRate:     item.TaxRate,
		})
	}
	out, err := h.sales.Create(c.Request.Context(), sales.CreateSaleInput{
		OrgID:         orgID,
		CustomerID:    customerID,
		CustomerName:  strings.TrimSpace(req.CustomerName),
		QuoteID:       quoteID,
		PaymentMethod: strings.TrimSpace(req.PaymentMethod),
		Items:         items,
		Notes:         strings.TrimSpace(req.Notes),
		CreatedBy:     "internal-service",
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) CreateSalePaymentLink(c *gin.Context) {
	orgID, err := uuid.Parse(strings.TrimSpace(c.GetHeader("X-Org-ID")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header required"})
		return
	}
	saleID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	pref, err := h.gateway.GetOrCreatePreference(c.Request.Context(), orgID, paymentgateway.CreatePreferenceRequest{
		ReferenceType: "sale",
		ReferenceID:   saleID,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":             pref.ID.String(),
		"provider":       pref.Provider,
		"reference_type": pref.ReferenceType,
		"reference_id":   pref.ReferenceID.String(),
		"status":         pref.Status,
		"amount":         pref.Amount,
		"payment_url":    pref.PaymentURL,
		"qr_data":        pref.QRData,
		"expires_at":     pref.ExpiresAt.UTC().Format(time.RFC3339),
		"created_at":     pref.CreatedAt.UTC().Format(time.RFC3339),
	})
}

// InternalSendWhatsAppText permite a servicios internos (p. ej. vertical talleres) enviar texto respetando opt-in en el core.
func (h *Handler) InternalSendWhatsAppText(c *gin.Context) {
	if h.whatsapp == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "whatsapp send unavailable"})
		return
	}
	orgID, err := uuid.Parse(strings.TrimSpace(c.GetHeader("X-Org-ID")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header required"})
		return
	}
	var req struct {
		PartyID string `json:"party_id" binding:"required"`
		Body    string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	partyID, err := uuid.Parse(strings.TrimSpace(req.PartyID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid party_id"})
		return
	}
	_, err = h.whatsapp.SendText(c.Request.Context(), wapdomain.SendTextRequest{
		OrgID:   orgID,
		PartyID: partyID,
		Body:    strings.TrimSpace(req.Body),
		Actor:   "internal-service:workshops",
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "sent"})
}
