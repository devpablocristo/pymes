package workorders

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/workorders/handler/dto"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/workorders/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalgin"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/vertvalues"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.WorkOrder, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.WorkOrder, actor string) (domain.WorkOrder, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.WorkOrder, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.WorkOrder, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.GET("/work-orders", h.List)
	authGroup.POST("/work-orders", h.Create)
	authGroup.GET("/work-orders/:id", h.Get)
	authGroup.PUT("/work-orders/:id", h.Update)
}

func (h *Handler) List(c *gin.Context) {
	orgID, ok := verticalgin.ParseAuthOrgID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	var after *uuid.UUID
	if value := c.Query("after"); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid after"})
			return
		}
		after = &parsed
	}
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{
		OrgID:  orgID,
		Limit:  limit,
		After:  after,
		Search: c.Query("search"),
		Status: c.Query("status"),
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListWorkOrdersResponse{Items: toWorkOrderItems(items), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	authCtx := auth.GetAuthContext(c)
	orgID, ok := verticalgin.ParseAuthOrgID(c)
	if !ok {
		return
	}
	var req dto.CreateWorkOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	vehicleID, err := uuid.Parse(strings.TrimSpace(req.VehicleID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid vehicle_id"})
		return
	}
	openedAt, err := verticalgin.ParseRFC3339(req.OpenedAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid opened_at"})
		return
	}
	promisedAt, err := verticalgin.ParseOptionalRFC3339(req.PromisedAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid promised_at"})
		return
	}
	out, err := h.uc.Create(c.Request.Context(), domain.WorkOrder{
		OrgID:         orgID,
		Number:        req.Number,
		VehicleID:     vehicleID,
		VehiclePlate:  req.VehiclePlate,
		CustomerID:    vertvalues.ParseOptionalUUID(req.CustomerID),
		CustomerName:  req.CustomerName,
		AppointmentID: vertvalues.ParseOptionalUUID(req.AppointmentID),
		Status:        req.Status,
		RequestedWork: req.RequestedWork,
		Diagnosis:     req.Diagnosis,
		Notes:         req.Notes,
		InternalNotes: req.InternalNotes,
		Currency:      req.Currency,
		OpenedAt:      openedAt,
		PromisedAt:    promisedAt,
		CreatedBy:     authCtx.Actor,
		Items:         toItems(req.Items),
	}, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toWorkOrderItem(out))
}

func (h *Handler) Get(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toWorkOrderItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateWorkOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	promisedAt, err := verticalgin.ParseOptionalRFC3339Ptr(req.PromisedAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid promised_at"})
		return
	}
	readyAt, err := verticalgin.ParseNullableRFC3339Ptr(req.ReadyAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ready_at"})
		return
	}
	deliveredAt, err := verticalgin.ParseNullableRFC3339Ptr(req.DeliveredAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid delivered_at"})
		return
	}
	var items *[]domain.WorkOrderItem
	if req.Items != nil {
		converted := toItems(*req.Items)
		items = &converted
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		VehicleID:     req.VehicleID,
		VehiclePlate:  req.VehiclePlate,
		CustomerID:    req.CustomerID,
		CustomerName:  req.CustomerName,
		AppointmentID: req.AppointmentID,
		Status:        req.Status,
		RequestedWork: req.RequestedWork,
		Diagnosis:     req.Diagnosis,
		Notes:         req.Notes,
		InternalNotes: req.InternalNotes,
		Currency:      req.Currency,
		PromisedAt:    promisedAt,
		ReadyAt:       readyAt,
		DeliveredAt:   deliveredAt,
		Items:         items,
	}, auth.GetAuthContext(c).Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toWorkOrderItem(out))
}

func toItems(payload []dto.WorkOrderLineInput) []domain.WorkOrderItem {
	items := make([]domain.WorkOrderItem, 0, len(payload))
	for _, item := range payload {
		items = append(items, domain.WorkOrderItem{
			ItemType:    item.ItemType,
			ServiceID:   vertvalues.ParseOptionalUUID(item.ServiceID),
			ProductID:   vertvalues.ParseOptionalUUID(item.ProductID),
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			TaxRate:     item.TaxRate,
			Metadata:    item.Metadata,
		})
	}
	return items
}

func toWorkOrderItems(items []domain.WorkOrder) []dto.WorkOrderItem {
	out := make([]dto.WorkOrderItem, 0, len(items))
	for _, item := range items {
		out = append(out, toWorkOrderItem(item))
	}
	return out
}

func toWorkOrderItem(item domain.WorkOrder) dto.WorkOrderItem {
	result := dto.WorkOrderItem{
		ID:               item.ID.String(),
		OrgID:            item.OrgID.String(),
		Number:           item.Number,
		VehicleID:        item.VehicleID.String(),
		VehiclePlate:     item.VehiclePlate,
		CustomerName:     item.CustomerName,
		Status:           item.Status,
		RequestedWork:    item.RequestedWork,
		Diagnosis:        item.Diagnosis,
		Notes:            item.Notes,
		InternalNotes:    item.InternalNotes,
		Currency:         item.Currency,
		SubtotalServices: item.SubtotalServices,
		SubtotalParts:    item.SubtotalParts,
		TaxTotal:         item.TaxTotal,
		Total:            item.Total,
		OpenedAt:         item.OpenedAt.UTC().Format(time.RFC3339),
		CreatedBy:        item.CreatedBy,
		CreatedAt:        item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        item.UpdatedAt.UTC().Format(time.RFC3339),
		Items:            toWorkOrderLineItems(item.Items),
	}
	if item.CustomerID != nil {
		value := item.CustomerID.String()
		result.CustomerID = &value
	}
	if item.AppointmentID != nil {
		value := item.AppointmentID.String()
		result.AppointmentID = &value
	}
	if item.QuoteID != nil {
		value := item.QuoteID.String()
		result.QuoteID = &value
	}
	if item.SaleID != nil {
		value := item.SaleID.String()
		result.SaleID = &value
	}
	if item.PromisedAt != nil {
		value := item.PromisedAt.UTC().Format(time.RFC3339)
		result.PromisedAt = &value
	}
	if item.ReadyAt != nil {
		value := item.ReadyAt.UTC().Format(time.RFC3339)
		result.ReadyAt = &value
	}
	if item.DeliveredAt != nil {
		value := item.DeliveredAt.UTC().Format(time.RFC3339)
		result.DeliveredAt = &value
	}
	return result
}

func toWorkOrderLineItems(items []domain.WorkOrderItem) []dto.WorkOrderLineItem {
	out := make([]dto.WorkOrderLineItem, 0, len(items))
	for _, item := range items {
		result := dto.WorkOrderLineItem{
			ID:          item.ID.String(),
			ItemType:    item.ItemType,
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			TaxRate:     item.TaxRate,
			SortOrder:   item.SortOrder,
			Metadata:    item.Metadata,
		}
		if item.ServiceID != nil {
			value := item.ServiceID.String()
			result.ServiceID = &value
		}
		if item.ProductID != nil {
			value := item.ProductID.String()
			result.ProductID = &value
		}
		out = append(out, result)
	}
	return out
}
