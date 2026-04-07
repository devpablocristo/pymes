package workorders

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	crudpaths "github.com/devpablocristo/modules/crud/paths/go/paths"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalgin"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/vertvalues"
	"github.com/devpablocristo/pymes/workshops/backend/internal/workorders/handler/dto"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/workorders/usecases/domain"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.WorkOrder, int64, bool, *uuid.UUID, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, targetType string) ([]domain.WorkOrder, error)
	Create(ctx context.Context, in domain.WorkOrder, actor string) (domain.WorkOrder, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.WorkOrder, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.WorkOrder, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

// RegisterRoutes monta el endpoint unificado /work-orders bajo el grupo auth recibido.
// El base path lo define el bootstrap (típicamente "/v1").
func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	const workOrdersBasePath = "/work-orders"
	const workOrdersItemPath = workOrdersBasePath + "/:id"

	authGroup.GET(workOrdersBasePath, h.List)
	authGroup.GET(workOrdersBasePath+"/"+crudpaths.SegmentArchived, h.ListArchived)
	authGroup.POST(workOrdersBasePath, h.Create)
	authGroup.GET(workOrdersItemPath, h.Get)
	authGroup.PUT(workOrdersItemPath, h.Update)
	authGroup.PATCH(workOrdersItemPath, h.Update)
	authGroup.DELETE(workOrdersItemPath, h.Delete)
	authGroup.POST(workOrdersItemPath+"/"+crudpaths.SegmentRestore, h.Restore)
	authGroup.DELETE(workOrdersItemPath+"/"+crudpaths.SegmentHard, h.HardDelete)
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
		OrgID:      orgID,
		Limit:      limit,
		After:      after,
		Search:     c.Query("search"),
		Status:     c.Query("status"),
		TargetType: c.Query("target_type"),
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

func (h *Handler) ListArchived(c *gin.Context) {
	orgID, ok := verticalgin.ParseAuthOrgID(c)
	if !ok {
		return
	}
	items, err := h.uc.ListArchived(c.Request.Context(), orgID, c.Query("target_type"))
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListWorkOrdersResponse{Items: toWorkOrderItems(items), Total: int64(len(items)), HasMore: false}
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

	targetType, targetIDRaw, targetLabel := resolveTargetFromCreate(req)
	if targetType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_type is required"})
		return
	}
	targetID, err := uuid.Parse(strings.TrimSpace(targetIDRaw))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target_id"})
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

	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	out, err := h.uc.Create(c.Request.Context(), domain.WorkOrder{
		OrgID:         orgID,
		Number:        req.Number,
		TargetType:    targetType,
		TargetID:      targetID,
		TargetLabel:   targetLabel,
		CustomerID:    vertvalues.ParseOptionalUUID(req.CustomerID),
		CustomerName:  req.CustomerName,
		BookingID:     vertvalues.ParseOptionalUUID(req.BookingID),
		Status:        req.Status,
		RequestedWork: req.RequestedWork,
		Diagnosis:     req.Diagnosis,
		Notes:         req.Notes,
		InternalNotes: req.InternalNotes,
		Currency:      req.Currency,
		OpenedAt:      openedAt,
		PromisedAt:    promisedAt,
		Metadata:      metadata,
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

	targetID, targetLabel := resolveTargetFromUpdate(req)
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		TargetID:      targetID,
		TargetLabel:   targetLabel,
		CustomerID:    req.CustomerID,
		CustomerName:  req.CustomerName,
		BookingID:     req.BookingID,
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

func (h *Handler) Delete(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.SoftDelete(c.Request.Context(), orgID, id, auth.GetAuthContext(c).Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Restore(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.Restore(c.Request.Context(), orgID, id, auth.GetAuthContext(c).Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HardDelete(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.HardDelete(c.Request.Context(), orgID, id, auth.GetAuthContext(c).Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// resolveTargetFromCreate detecta target_type, target_id, target_label desde el request,
// soportando tanto la forma unificada como los aliases legacy.
func resolveTargetFromCreate(req dto.CreateWorkOrderRequest) (string, string, string) {
	if t := strings.TrimSpace(req.TargetType); t != "" {
		return t, req.TargetID, req.TargetLabel
	}
	if id := strings.TrimSpace(req.VehicleID); id != "" {
		return "vehicle", id, req.VehiclePlate
	}
	if id := strings.TrimSpace(req.BicycleID); id != "" {
		return "bicycle", id, req.BicycleLabel
	}
	return "", "", ""
}

func resolveTargetFromUpdate(req dto.UpdateWorkOrderRequest) (*string, *string) {
	if req.TargetID != nil {
		return req.TargetID, req.TargetLabel
	}
	if req.VehicleID != nil {
		return req.VehicleID, req.VehiclePlate
	}
	if req.BicycleID != nil {
		return req.BicycleID, req.BicycleLabel
	}
	return nil, req.TargetLabel
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
		TargetType:       item.TargetType,
		TargetID:         item.TargetID.String(),
		TargetLabel:      item.TargetLabel,
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
		Metadata:         item.Metadata,
		CreatedBy:        item.CreatedBy,
		CreatedAt:        item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        item.UpdatedAt.UTC().Format(time.RFC3339),
		Items:            toWorkOrderLineItems(item.Items),
	}
	// Aliases por compat: solo se llenan según target_type.
	switch item.TargetType {
	case "vehicle":
		result.VehicleID = item.TargetID.String()
		result.VehiclePlate = item.TargetLabel
	case "bicycle":
		result.BicycleID = item.TargetID.String()
		result.BicycleLabel = item.TargetLabel
	}
	if item.CustomerID != nil {
		value := item.CustomerID.String()
		result.CustomerID = &value
	}
	if item.BookingID != nil {
		value := item.BookingID.String()
		result.BookingID = &value
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
	if item.ReadyPickupNotifiedAt != nil {
		value := item.ReadyPickupNotifiedAt.UTC().Format(time.RFC3339)
		result.ReadyPickupNotifiedAt = &value
	}
	if item.ArchivedAt != nil {
		s := item.ArchivedAt.UTC().Format(time.RFC3339)
		result.ArchivedAt = &s
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
