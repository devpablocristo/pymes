package workorders

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/devpablocristo/pymes/workshops/backend/internal/workorders/repository/models"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/workorders/usecases/domain"
)

// ErrWorkOrderHasIntegrations bloquea borrado en duro si hay presupuesto o venta vinculados.
var ErrWorkOrderHasIntegrations = errors.New("work order has quote or sale linked")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// List devuelve una página de OTs no archivadas, opcionalmente filtradas por target_type/status/search.
func (r *Repository) List(ctx context.Context, p ListParams) ([]domain.WorkOrder, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 250})
	q := r.db.WithContext(ctx).Model(&models.WorkOrderModel{}).Where("org_id = ? AND archived_at IS NULL", p.OrgID)
	if p.BranchID != nil && *p.BranchID != uuid.Nil {
		// Durante la migración conviene seguir mostrando OTs legacy sin branch asignada.
		q = q.Where("(branch_id = ? OR branch_id IS NULL)", *p.BranchID)
	}
	if targetType := strings.TrimSpace(p.TargetType); targetType != "" {
		q = q.Where("target_type = ?", targetType)
	}
	if status := strings.TrimSpace(p.Status); status != "" {
		q = q.Where("status = ?", status)
	}
	if search := strings.TrimSpace(p.Search); search != "" {
		like := "%" + search + "%"
		q = q.Where("(number ILIKE ? OR target_label ILIKE ? OR customer_name ILIKE ? OR requested_work ILIKE ?)", like, like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}
	if p.After != nil && *p.After != uuid.Nil {
		q = q.Where("id < ?", *p.After)
	}
	q = q.Order("id DESC")

	var rows []models.WorkOrderModel
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]domain.WorkOrder, 0, len(rows))
	for _, row := range rows {
		items, err := r.loadItems(ctx, row.OrgID, row.ID)
		if err != nil {
			return nil, 0, false, nil, err
		}
		out = append(out, toDomain(row, items))
	}
	var next *uuid.UUID
	if hasMore && len(rows) > 0 {
		value := rows[len(rows)-1].ID
		next = &value
	}
	return out, total, hasMore, next, nil
}

func (r *Repository) ListArchived(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, targetType string) ([]domain.WorkOrder, error) {
	q := r.db.WithContext(ctx).
		Model(&models.WorkOrderModel{}).
		Where("org_id = ? AND archived_at IS NOT NULL", orgID)
	if branchID != nil && *branchID != uuid.Nil {
		q = q.Where("(branch_id = ? OR branch_id IS NULL)", *branchID)
	}
	if t := strings.TrimSpace(targetType); t != "" {
		q = q.Where("target_type = ?", t)
	}
	var rows []models.WorkOrderModel
	if err := q.Order("updated_at DESC").Limit(200).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.WorkOrder, 0, len(rows))
	for _, row := range rows {
		items, err := r.loadItems(ctx, row.OrgID, row.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, toDomain(row, items))
	}
	return out, nil
}

func (r *Repository) Create(ctx context.Context, in domain.WorkOrder) (domain.WorkOrder, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		metadata, _ := json.Marshal(in.Metadata)
		row := models.WorkOrderModel{
			ID:               uuid.New(),
			OrgID:            in.OrgID,
			BranchID:         in.BranchID,
			Number:           in.Number,
			TargetType:       in.TargetType,
			TargetID:         in.TargetID,
			TargetLabel:      in.TargetLabel,
			CustomerID:       in.CustomerID,
			CustomerName:     in.CustomerName,
			BookingID:        in.BookingID,
			QuoteID:          in.QuoteID,
			SaleID:           in.SaleID,
			Status:           in.Status,
			RequestedWork:    in.RequestedWork,
			Diagnosis:        in.Diagnosis,
			Notes:            in.Notes,
			InternalNotes:    in.InternalNotes,
			Currency:         in.Currency,
			SubtotalServices: in.SubtotalServices,
			SubtotalParts:    in.SubtotalParts,
			TaxTotal:         in.TaxTotal,
			Total:            in.Total,
			OpenedAt:         in.OpenedAt,
			PromisedAt:       in.PromisedAt,
			ReadyAt:          in.ReadyAt,
			DeliveredAt:      in.DeliveredAt,
			Metadata:         metadata,
			CreatedBy:        in.CreatedBy,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}
		in.ID = row.ID
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return r.replaceItems(ctx, tx, in.OrgID, in.ID, in.Items)
	})
	if err != nil {
		return domain.WorkOrder{}, err
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.WorkOrder, error) {
	var row models.WorkOrderModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.WorkOrder{}, gorm.ErrRecordNotFound
		}
		return domain.WorkOrder{}, err
	}
	items, err := r.loadItems(ctx, orgID, id)
	if err != nil {
		return domain.WorkOrder{}, err
	}
	return toDomain(row, items), nil
}

func (r *Repository) Update(ctx context.Context, in domain.WorkOrder) (domain.WorkOrder, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		metadata, _ := json.Marshal(in.Metadata)
		updates := map[string]any{
			"branch_id":         in.BranchID,
			"target_id":         in.TargetID,
			"target_label":      in.TargetLabel,
			"customer_id":       in.CustomerID,
			"customer_name":     in.CustomerName,
			"booking_id":        in.BookingID,
			"quote_id":          in.QuoteID,
			"sale_id":           in.SaleID,
			"status":            in.Status,
			"requested_work":    in.RequestedWork,
			"diagnosis":         in.Diagnosis,
			"notes":             in.Notes,
			"internal_notes":    in.InternalNotes,
			"currency":          in.Currency,
			"subtotal_services": in.SubtotalServices,
			"subtotal_parts":    in.SubtotalParts,
			"tax_total":         in.TaxTotal,
			"total":             in.Total,
			"opened_at":         in.OpenedAt,
			"promised_at":       in.PromisedAt,
			"ready_at":          in.ReadyAt,
			"delivered_at":      in.DeliveredAt,
			"metadata":          metadata,
			"updated_at":        time.Now().UTC(),
		}
		res := tx.Model(&models.WorkOrderModel{}).Where("org_id = ? AND id = ? AND archived_at IS NULL", in.OrgID, in.ID).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return r.replaceItems(ctx, tx, in.OrgID, in.ID, in.Items)
	})
	if err != nil {
		return domain.WorkOrder{}, err
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func (r *Repository) SaveIntegrations(ctx context.Context, orgID, id uuid.UUID, quoteID, saleID *uuid.UUID, status *string) (domain.WorkOrder, error) {
	updates := map[string]any{"updated_at": time.Now().UTC()}
	if quoteID != nil {
		updates["quote_id"] = quoteID
	}
	if saleID != nil {
		updates["sale_id"] = saleID
	}
	if status != nil {
		updates["status"] = *status
	}
	res := r.db.WithContext(ctx).Model(&models.WorkOrderModel{}).Where("org_id = ? AND id = ? AND archived_at IS NULL", orgID, id).Updates(updates)
	if res.Error != nil {
		return domain.WorkOrder{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.WorkOrder{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, orgID, id)
}

func (r *Repository) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&models.WorkOrderModel{}).
		Where("org_id = ? AND id = ? AND archived_at IS NULL", orgID, id).
		Updates(map[string]any{"archived_at": now, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) Restore(ctx context.Context, orgID, id uuid.UUID) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&models.WorkOrderModel{}).
		Where("org_id = ? AND id = ? AND archived_at IS NOT NULL", orgID, id).
		Updates(map[string]any{"archived_at": nil, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) HardDelete(ctx context.Context, orgID, id uuid.UUID) error {
	var row models.WorkOrderModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		return err
	}
	if row.ArchivedAt == nil {
		return gorm.ErrRecordNotFound
	}
	if row.QuoteID != nil || row.SaleID != nil {
		return ErrWorkOrderHasIntegrations
	}
	res := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Delete(&models.WorkOrderModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) replaceItems(ctx context.Context, tx *gorm.DB, orgID, workOrderID uuid.UUID, items []domain.WorkOrderItem) error {
	if err := tx.WithContext(ctx).Where("org_id = ? AND work_order_id = ?", orgID, workOrderID).Delete(&models.WorkOrderItemModel{}).Error; err != nil {
		return err
	}
	for index, item := range items {
		metadata, _ := json.Marshal(item.Metadata)
		row := models.WorkOrderItemModel{
			ID:          uuid.New(),
			OrgID:       orgID,
			WorkOrderID: workOrderID,
			ItemType:    item.ItemType,
			ServiceID:   item.ServiceID,
			ProductID:   item.ProductID,
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			TaxRate:     item.TaxRate,
			SortOrder:   index,
			Metadata:    metadata,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) loadItems(ctx context.Context, orgID, workOrderID uuid.UUID) ([]domain.WorkOrderItem, error) {
	var rows []models.WorkOrderItemModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND work_order_id = ?", orgID, workOrderID).
		Order("sort_order ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]domain.WorkOrderItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, itemToDomain(row))
	}
	return items, nil
}

func itemToDomain(row models.WorkOrderItemModel) domain.WorkOrderItem {
	metadata := map[string]any{}
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &metadata)
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	return domain.WorkOrderItem{
		ID:          row.ID,
		OrgID:       row.OrgID,
		WorkOrderID: row.WorkOrderID,
		ItemType:    row.ItemType,
		ServiceID:   row.ServiceID,
		ProductID:   row.ProductID,
		Description: row.Description,
		Quantity:    row.Quantity,
		UnitPrice:   row.UnitPrice,
		TaxRate:     row.TaxRate,
		SortOrder:   row.SortOrder,
		Metadata:    metadata,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func toDomain(row models.WorkOrderModel, items []domain.WorkOrderItem) domain.WorkOrder {
	metadata := map[string]any{}
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &metadata)
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	return domain.WorkOrder{
		ID:               row.ID,
		OrgID:            row.OrgID,
		BranchID:         row.BranchID,
		Number:           row.Number,
		TargetType:       row.TargetType,
		TargetID:         row.TargetID,
		TargetLabel:      row.TargetLabel,
		CustomerID:       row.CustomerID,
		CustomerName:     row.CustomerName,
		BookingID:        row.BookingID,
		QuoteID:          row.QuoteID,
		SaleID:           row.SaleID,
		Status:           row.Status,
		RequestedWork:    row.RequestedWork,
		Diagnosis:        row.Diagnosis,
		Notes:            row.Notes,
		InternalNotes:    row.InternalNotes,
		Currency:         row.Currency,
		SubtotalServices: row.SubtotalServices,
		SubtotalParts:    row.SubtotalParts,
		TaxTotal:         row.TaxTotal,
		Total:            row.Total,
		OpenedAt:         row.OpenedAt,
		PromisedAt:       row.PromisedAt,
		ReadyAt:          row.ReadyAt,
		DeliveredAt:      row.DeliveredAt,
		Metadata:         metadata,
		CreatedBy:        row.CreatedBy,
		ArchivedAt:       row.ArchivedAt,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Items:            items,
	}
}
