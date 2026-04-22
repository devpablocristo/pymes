package invoices

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	utils "github.com/devpablocristo/core/validate/go/stringutil"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/invoices/repository/models"
	invdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/invoices/usecases/domain"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type ListParams struct {
	OrgID  uuid.UUID
	Limit  int
	After  *uuid.UUID
	Status string
}

func (r *Repository) List(ctx context.Context, p ListParams) ([]invdomain.Invoice, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&models.InvoiceModel{}).
		Where("org_id = ? AND deleted_at IS NULL", p.OrgID)
	if s := strings.TrimSpace(p.Status); s != "" {
		q = q.Where("status = ?", s)
	}
	if p.After != nil && *p.After != uuid.Nil {
		q = q.Where("id < ?", *p.After)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}
	var rows []models.InvoiceModel
	if err := q.Order("created_at DESC").Order("id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out, err := r.hydrateWithItems(ctx, rows)
	if err != nil {
		return nil, 0, false, nil, err
	}
	var next *uuid.UUID
	if hasMore && len(rows) > 0 {
		v := rows[len(rows)-1].ID
		next = &v
	}
	return out, total, hasMore, next, nil
}

func (r *Repository) ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]invdomain.Invoice, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	var rows []models.InvoiceModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND deleted_at IS NOT NULL", orgID).
		Order("deleted_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return r.hydrateWithItems(ctx, rows)
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (invdomain.Invoice, error) {
	var row models.InvoiceModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).
		Take(&row).Error; err != nil {
		return invdomain.Invoice{}, err
	}
	out, err := r.hydrateWithItems(ctx, []models.InvoiceModel{row})
	if err != nil {
		return invdomain.Invoice{}, err
	}
	return out[0], nil
}

func (r *Repository) Create(ctx context.Context, in invdomain.Invoice) (invdomain.Invoice, error) {
	id := uuid.New()
	now := time.Now().UTC()
	number := strings.TrimSpace(in.Number)
	if number == "" {
		number = defaultInvoiceNumber(now)
	}
	row := models.InvoiceModel{
		ID:              id,
		OrgID:           in.OrgID,
		Number:          number,
		PartyID:         in.PartyID,
		CustomerName:    strings.TrimSpace(in.CustomerName),
		IssuedDate:      in.IssuedDate,
		DueDate:         in.DueDate,
		Status:          string(in.Status),
		Subtotal:        in.Subtotal,
		DiscountPercent: in.DiscountPercent,
		TaxPercent:      in.TaxPercent,
		Total:           in.Total,
		Notes:           strings.TrimSpace(in.Notes),
		IsFavorite:      in.IsFavorite,
		Tags:            pq.StringArray(utils.NormalizeTags(in.Tags)),
		CreatedBy:       strings.TrimSpace(in.CreatedBy),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		for _, item := range in.Items {
			im := models.InvoiceLineItemModel{
				ID:          uuid.New(),
				InvoiceID:   id,
				Description: strings.TrimSpace(item.Description),
				Qty:         item.Qty,
				Unit:        strings.TrimSpace(item.Unit),
				UnitPrice:   item.UnitPrice,
				LineTotal:   item.LineTotal,
				SortOrder:   item.SortOrder,
			}
			if im.Unit == "" {
				im.Unit = "unidad"
			}
			if err := tx.Create(&im).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return invdomain.Invoice{}, err
	}
	return r.GetByID(ctx, in.OrgID, id)
}

func (r *Repository) Update(ctx context.Context, in invdomain.Invoice) (invdomain.Invoice, error) {
	now := time.Now().UTC()
	updates := map[string]any{
		"status":           string(in.Status),
		"discount_percent": in.DiscountPercent,
		"tax_percent":      in.TaxPercent,
		"subtotal":         in.Subtotal,
		"total":            in.Total,
		"notes":            strings.TrimSpace(in.Notes),
		"is_favorite":      in.IsFavorite,
		"tags":             pq.StringArray(utils.NormalizeTags(in.Tags)),
		"issued_date":      in.IssuedDate,
		"due_date":         in.DueDate,
		"updated_at":       now,
	}
	res := r.db.WithContext(ctx).Model(&models.InvoiceModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return invdomain.Invoice{}, res.Error
	}
	if res.RowsAffected == 0 {
		return invdomain.Invoice{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func (r *Repository) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&models.InvoiceModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).
		Update("deleted_at", now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) Restore(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&models.InvoiceModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NOT NULL", orgID, id).
		Update("deleted_at", nil)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) HardDelete(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Delete(&models.InvoiceModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) hydrateWithItems(ctx context.Context, rows []models.InvoiceModel) ([]invdomain.Invoice, error) {
	if len(rows) == 0 {
		return []invdomain.Invoice{}, nil
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	var items []models.InvoiceLineItemModel
	if err := r.db.WithContext(ctx).
		Where("invoice_id IN ?", ids).
		Order("sort_order ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	byInvoice := make(map[uuid.UUID][]invdomain.InvoiceLineItem, len(rows))
	for _, it := range items {
		byInvoice[it.InvoiceID] = append(byInvoice[it.InvoiceID], invdomain.InvoiceLineItem{
			ID:          it.ID,
			InvoiceID:   it.InvoiceID,
			Description: it.Description,
			Qty:         it.Qty,
			Unit:        it.Unit,
			UnitPrice:   it.UnitPrice,
			LineTotal:   it.LineTotal,
			SortOrder:   it.SortOrder,
		})
	}
	for _, list := range byInvoice {
		sort.SliceStable(list, func(i, j int) bool { return list[i].SortOrder < list[j].SortOrder })
	}
	out := make([]invdomain.Invoice, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row, byInvoice[row.ID]))
	}
	return out, nil
}

func toDomain(row models.InvoiceModel, items []invdomain.InvoiceLineItem) invdomain.Invoice {
	if items == nil {
		items = []invdomain.InvoiceLineItem{}
	}
	return invdomain.Invoice{
		ID:              row.ID,
		OrgID:           row.OrgID,
		Number:          row.Number,
		PartyID:         row.PartyID,
		CustomerName:    row.CustomerName,
		IssuedDate:      row.IssuedDate,
		DueDate:         row.DueDate,
		Status:          invdomain.InvoiceStatus(row.Status),
		Subtotal:        row.Subtotal,
		DiscountPercent: row.DiscountPercent,
		TaxPercent:      row.TaxPercent,
		Total:           row.Total,
		Notes:           row.Notes,
		IsFavorite:      row.IsFavorite,
		Tags:            append([]string(nil), row.Tags...),
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		ArchivedAt:      row.DeletedAt,
		Items:           items,
	}
}

func defaultInvoiceNumber(now time.Time) string {
	return "INV-" + now.UTC().Format("20060102-150405")
}
