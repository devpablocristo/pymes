package workorders

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
	sharedhandlers "github.com/devpablocristo/pymes/workshops/backend/internal/shared/handlers"
	"github.com/devpablocristo/pymes/workshops/backend/internal/shared/values"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]WorkOrder, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in WorkOrder, actor string) (WorkOrder, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (WorkOrder, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (WorkOrder, error)
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
	orgID, ok := sharedhandlers.ParseAuthOrgID(c)
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
	resp := gin.H{"items": toWorkOrderItems(items), "total": total, "has_more": hasMore}
	if next != nil {
		resp["next_cursor"] = next.String()
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	authCtx := auth.GetAuthContext(c)
	orgID, ok := sharedhandlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	var req struct {
		Number        string        `json:"number"`
		VehicleID     string        `json:"vehicle_id" binding:"required"`
		VehiclePlate  string        `json:"vehicle_plate"`
		CustomerID    string        `json:"customer_id"`
		CustomerName  string        `json:"customer_name"`
		AppointmentID string        `json:"appointment_id"`
		Status        string        `json:"status"`
		RequestedWork string        `json:"requested_work"`
		Diagnosis     string        `json:"diagnosis"`
		Notes         string        `json:"notes"`
		InternalNotes string        `json:"internal_notes"`
		Currency      string        `json:"currency"`
		OpenedAt      string        `json:"opened_at"`
		PromisedAt    string        `json:"promised_at"`
		Items         []itemPayload `json:"items" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	vehicleID, err := uuid.Parse(strings.TrimSpace(req.VehicleID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid vehicle_id"})
		return
	}
	openedAt, err := sharedhandlers.ParseRFC3339(req.OpenedAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid opened_at"})
		return
	}
	promisedAt, err := sharedhandlers.ParseOptionalRFC3339(req.PromisedAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid promised_at"})
		return
	}
	out, err := h.uc.Create(c.Request.Context(), WorkOrder{
		OrgID:         orgID,
		Number:        req.Number,
		VehicleID:     vehicleID,
		VehiclePlate:  req.VehiclePlate,
		CustomerID:    values.ParseOptionalUUID(req.CustomerID),
		CustomerName:  req.CustomerName,
		AppointmentID: values.ParseOptionalUUID(req.AppointmentID),
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
	orgID, id, ok := sharedhandlers.ParseAuthOrgAndParamID(c, "id", "id")
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
	orgID, id, ok := sharedhandlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req struct {
		VehicleID     *string        `json:"vehicle_id"`
		VehiclePlate  *string        `json:"vehicle_plate"`
		CustomerID    *string        `json:"customer_id"`
		CustomerName  *string        `json:"customer_name"`
		AppointmentID *string        `json:"appointment_id"`
		Status        *string        `json:"status"`
		RequestedWork *string        `json:"requested_work"`
		Diagnosis     *string        `json:"diagnosis"`
		Notes         *string        `json:"notes"`
		InternalNotes *string        `json:"internal_notes"`
		Currency      *string        `json:"currency"`
		PromisedAt    *string        `json:"promised_at"`
		ReadyAt       *string        `json:"ready_at"`
		DeliveredAt   *string        `json:"delivered_at"`
		Items         *[]itemPayload `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	promisedAt, err := sharedhandlers.ParseOptionalRFC3339Ptr(req.PromisedAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid promised_at"})
		return
	}
	readyAt, err := sharedhandlers.ParseNullableRFC3339Ptr(req.ReadyAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ready_at"})
		return
	}
	deliveredAt, err := sharedhandlers.ParseNullableRFC3339Ptr(req.DeliveredAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid delivered_at"})
		return
	}
	var items *[]WorkOrderItem
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

type itemPayload struct {
	ItemType    string         `json:"item_type"`
	ServiceID   string         `json:"service_id"`
	ProductID   string         `json:"product_id"`
	Description string         `json:"description"`
	Quantity    float64        `json:"quantity"`
	UnitPrice   float64        `json:"unit_price"`
	TaxRate     float64        `json:"tax_rate"`
	Metadata    map[string]any `json:"metadata"`
}

func toItems(payload []itemPayload) []WorkOrderItem {
	items := make([]WorkOrderItem, 0, len(payload))
	for _, item := range payload {
		items = append(items, WorkOrderItem{
			ItemType:    item.ItemType,
			ServiceID:   values.ParseOptionalUUID(item.ServiceID),
			ProductID:   values.ParseOptionalUUID(item.ProductID),
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			TaxRate:     item.TaxRate,
			Metadata:    item.Metadata,
		})
	}
	return items
}

func toWorkOrderItems(items []WorkOrder) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, toWorkOrderItem(item))
	}
	return out
}

func toWorkOrderItem(item WorkOrder) gin.H {
	result := gin.H{
		"id":                item.ID.String(),
		"org_id":            item.OrgID.String(),
		"number":            item.Number,
		"vehicle_id":        item.VehicleID.String(),
		"vehicle_plate":     item.VehiclePlate,
		"customer_name":     item.CustomerName,
		"status":            item.Status,
		"requested_work":    item.RequestedWork,
		"diagnosis":         item.Diagnosis,
		"notes":             item.Notes,
		"internal_notes":    item.InternalNotes,
		"currency":          item.Currency,
		"subtotal_services": item.SubtotalServices,
		"subtotal_parts":    item.SubtotalParts,
		"tax_total":         item.TaxTotal,
		"total":             item.Total,
		"opened_at":         item.OpenedAt.UTC().Format(time.RFC3339),
		"created_by":        item.CreatedBy,
		"created_at":        item.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":        item.UpdatedAt.UTC().Format(time.RFC3339),
		"items":             toWorkOrderLineItems(item.Items),
	}
	if item.CustomerID != nil {
		result["customer_id"] = item.CustomerID.String()
	}
	if item.AppointmentID != nil {
		result["appointment_id"] = item.AppointmentID.String()
	}
	if item.QuoteID != nil {
		result["quote_id"] = item.QuoteID.String()
	}
	if item.SaleID != nil {
		result["sale_id"] = item.SaleID.String()
	}
	if item.PromisedAt != nil {
		result["promised_at"] = item.PromisedAt.UTC().Format(time.RFC3339)
	}
	if item.ReadyAt != nil {
		result["ready_at"] = item.ReadyAt.UTC().Format(time.RFC3339)
	}
	if item.DeliveredAt != nil {
		result["delivered_at"] = item.DeliveredAt.UTC().Format(time.RFC3339)
	}
	return result
}

func toWorkOrderLineItems(items []WorkOrderItem) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		result := gin.H{
			"id":          item.ID.String(),
			"item_type":   item.ItemType,
			"description": item.Description,
			"quantity":    item.Quantity,
			"unit_price":  item.UnitPrice,
			"tax_rate":    item.TaxRate,
			"sort_order":  item.SortOrder,
			"metadata":    item.Metadata,
		}
		if item.ServiceID != nil {
			result["service_id"] = item.ServiceID.String()
		}
		if item.ProductID != nil {
			result["product_id"] = item.ProductID.String()
		}
		out = append(out, result)
	}
	return out
}
