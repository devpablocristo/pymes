package currency

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/currency/handler/dto"
	currencydomain "github.com/devpablocristo/pymes/control-plane/backend/internal/currency/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/backend/pkg/http/errors"
)

type usecasesPort interface {
	ListLatest(ctx context.Context, orgID uuid.UUID, fromCurrency, toCurrency string, limit int) ([]currencydomain.ExchangeRate, error)
	Upsert(ctx context.Context, in currencydomain.ExchangeRate) (currencydomain.ExchangeRate, error)
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/exchange-rates", rbac.RequirePermission("currency", "read"), h.List)
	auth.POST("/exchange-rates", rbac.RequirePermission("currency", "update"), h.Upsert)
}

func (h *Handler) List(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, err := h.uc.ListLatest(c.Request.Context(), orgID, strings.TrimSpace(c.Query("from_currency")), strings.TrimSpace(c.Query("to_currency")), limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) Upsert(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	var req dto.CreateExchangeRateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rateDate := time.Now().UTC()
	if strings.TrimSpace(req.RateDate) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(req.RateDate))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rate_date"})
			return
		}
		rateDate = parsed.UTC()
	}
	out, err := h.uc.Upsert(c.Request.Context(), currencydomain.ExchangeRate{
		OrgID:        orgID,
		FromCurrency: strings.ToUpper(strings.TrimSpace(req.FromCurrency)),
		ToCurrency:   strings.ToUpper(strings.TrimSpace(req.ToCurrency)),
		RateType:     strings.ToLower(strings.TrimSpace(req.RateType)),
		BuyRate:      req.BuyRate,
		SellRate:     req.SellRate,
		Source:       strings.ToLower(strings.TrimSpace(req.Source)),
		RateDate:     rateDate,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}
