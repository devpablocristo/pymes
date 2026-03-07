package pdfgen

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pkgs/go-pkg/httperrors"
)

type usecasesPort interface {
	RenderQuotePDF(ctx context.Context, orgID, quoteID uuid.UUID) ([]byte, string, error)
	RenderSaleReceipt(ctx context.Context, orgID, saleID uuid.UUID) ([]byte, string, error)
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/quotes/:id/pdf", rbac.RequirePermission("quotes", "read"), h.QuotePDF)
	auth.GET("/sales/:id/receipt", rbac.RequirePermission("sales", "read"), h.SaleReceipt)
}

func (h *Handler) QuotePDF(c *gin.Context) {
	orgID, id, ok := parseIDs(c)
	if !ok {
		return
	}
	pdfBytes, filename, err := h.uc.RenderQuotePDF(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", `inline; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

func (h *Handler) SaleReceipt(c *gin.Context) {
	orgID, id, ok := parseIDs(c)
	if !ok {
		return
	}
	pdfBytes, filename, err := h.uc.RenderSaleReceipt(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", `inline; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

func parseIDs(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, id, true
}
