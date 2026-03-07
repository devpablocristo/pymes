package customers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/customers/handler/dto"
	customerdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/customers/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]customerdomain.Customer, int64, bool, *uuid.UUID, error)
	ListArchived(ctx context.Context, orgID uuid.UUID) ([]customerdomain.Customer, error)
	Create(ctx context.Context, in customerdomain.Customer, actor string) (customerdomain.Customer, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (customerdomain.Customer, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (customerdomain.Customer, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
	ListSales(ctx context.Context, orgID, customerID uuid.UUID) ([]customerdomain.SaleHistoryItem, error)
	ExportCSV(ctx context.Context, orgID uuid.UUID) ([]byte, error)
	ImportCSV(ctx context.Context, orgID uuid.UUID, csvData []byte, actor string) (int, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/customers", rbac.RequirePermission("customers", "read"), h.List)
	auth.GET("/customers/archived", rbac.RequirePermission("customers", "read"), h.ListArchived)
	auth.POST("/customers", rbac.RequirePermission("customers", "create"), h.Create)
	auth.GET("/customers/export", rbac.RequirePermission("customers", "export"), h.ExportCSV)
	auth.POST("/customers/import", rbac.RequirePermission("customers", "import"), h.ImportCSV)
	auth.GET("/customers/:id", rbac.RequirePermission("customers", "read"), h.Get)
	auth.PUT("/customers/:id", rbac.RequirePermission("customers", "update"), h.Update)
	auth.DELETE("/customers/:id", rbac.RequirePermission("customers", "delete"), h.Delete)
	auth.POST("/customers/:id/restore", rbac.RequirePermission("customers", "delete"), h.Restore)
	auth.DELETE("/customers/:id/hard", rbac.RequirePermission("customers", "delete"), h.HardDelete)
	auth.GET("/customers/:id/sales", rbac.RequirePermission("customers", "read"), h.SalesHistory)
}

func (h *Handler) List(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	var after *uuid.UUID
	if v := strings.TrimSpace(c.Query("after")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid after"})
			return
		}
		after = &id
	}
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{
		OrgID:  orgID,
		Limit:  limit,
		After:  after,
		Search: c.Query("search"),
		Type:   c.Query("type"),
		Tag:    c.Query("tag"),
		Sort:   c.Query("sort"),
		Order:  c.Query("order"),
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListCustomersResponse{Items: make([]dto.CustomerItem, 0, len(items)), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toCustomerItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	var req dto.CreateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := h.uc.Create(c.Request.Context(), customerdomain.Customer{
		OrgID:   orgID,
		Type:    req.Type,
		Name:    req.Name,
		TaxID:   req.TaxID,
		Email:   req.Email,
		Phone:   req.Phone,
		Address: toDomainAddress(req.Address),
		Notes:   req.Notes,
		Tags:    req.Tags,
		Metadata: func() map[string]any {
			if req.Metadata == nil {
				return map[string]any{}
			}
			return req.Metadata
		}(),
	}, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toCustomerItem(out))
}

func (h *Handler) Get(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toCustomerItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req dto.UpdateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var addr *customerdomain.Address
	if req.Address != nil {
		a := toDomainAddress(*req.Address)
		addr = &a
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		Type:     req.Type,
		Name:     req.Name,
		TaxID:    req.TaxID,
		Email:    req.Email,
		Phone:    req.Phone,
		Address:  addr,
		Notes:    req.Notes,
		Tags:     req.Tags,
		Metadata: req.Metadata,
	}, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toCustomerItem(out))
}

func (h *Handler) Delete(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.uc.SoftDelete(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListArchived(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	items, err := h.uc.ListArchived(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListCustomersResponse{Items: make([]dto.CustomerItem, 0, len(items)), Total: int64(len(items))}
	for _, it := range items {
		resp.Items = append(resp.Items, toCustomerItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Restore(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.uc.Restore(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HardDelete(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.uc.HardDelete(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) SalesHistory(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	items, err := h.uc.ListSales(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListSalesHistoryResponse{Items: make([]dto.SaleHistoryItem, 0, len(items))}
	for _, it := range items {
		resp.Items = append(resp.Items, dto.SaleHistoryItem{
			ID:            it.ID.String(),
			Number:        it.Number,
			Status:        it.Status,
			PaymentMethod: it.PaymentMethod,
			Total:         it.Total,
			Currency:      it.Currency,
			CreatedAt:     it.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ExportCSV(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	out, err := h.uc.ExportCSV(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="customers.csv"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", out)
}

func (h *Handler) ImportCSV(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	body, err := c.GetRawData()
	if err != nil || len(body) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "csv body required"})
		return
	}
	n, err := h.uc.ImportCSV(c.Request.Context(), orgID, body, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ImportCustomersResponse{Imported: n})
}

func toCustomerItem(in customerdomain.Customer) dto.CustomerItem {
	return dto.CustomerItem{
		ID:    in.ID.String(),
		OrgID: in.OrgID.String(),
		Type:  in.Type,
		Name:  in.Name,
		TaxID: in.TaxID,
		Email: in.Email,
		Phone: in.Phone,
		Address: dto.Address{
			Street:  in.Address.Street,
			City:    in.Address.City,
			State:   in.Address.State,
			ZipCode: in.Address.ZipCode,
			Country: in.Address.Country,
		},
		Notes:     in.Notes,
		Tags:      in.Tags,
		Metadata:  in.Metadata,
		CreatedAt: in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: in.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toDomainAddress(in dto.Address) customerdomain.Address {
	return customerdomain.Address{Street: strings.TrimSpace(in.Street), City: strings.TrimSpace(in.City), State: strings.TrimSpace(in.State), ZipCode: strings.TrimSpace(in.ZipCode), Country: strings.TrimSpace(in.Country)}
}
