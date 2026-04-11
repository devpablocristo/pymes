package inventory

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/inventory/repository/models"
	inventorydomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/inventory/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type ListStockParams struct {
	OrgID    uuid.UUID
	Limit    int
	After    *uuid.UUID
	LowStock bool
	Archived bool
	Order    string
}

type ListMovementParams struct {
	OrgID     uuid.UUID
	Limit     int
	After     *uuid.UUID
	ProductID *uuid.UUID
	Type      string
}

type productRow struct {
	ID         uuid.UUID
	Name       string
	SKU        string
	TrackStock bool `gorm:"column:track_stock"`
}

func (r *Repository) EnsureStockLevel(ctx context.Context, orgID, productID uuid.UUID) error {
	row := models.StockLevelModel{OrgID: orgID, ProductID: productID, Quantity: 0, MinQuantity: 0, UpdatedAt: time.Now().UTC()}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "org_id"}, {Name: "product_id"}}, DoNothing: true}).Create(&row).Error
}

func (r *Repository) GetProduct(ctx context.Context, orgID, productID uuid.UUID) (productRow, error) {
	var row productRow
	err := r.db.WithContext(ctx).
		Table("products").
		Select("id, name, sku, track_stock").
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, productID).
		Take(&row).Error
	if err != nil {
		return productRow{}, err
	}
	return row, nil
}

func (r *Repository) GetLevel(ctx context.Context, orgID, productID uuid.UUID) (inventorydomain.StockLevel, error) {
	p, err := r.GetProduct(ctx, orgID, productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return inventorydomain.StockLevel{}, gorm.ErrRecordNotFound
		}
		return inventorydomain.StockLevel{}, err
	}
	if !p.TrackStock {
		return inventorydomain.StockLevel{ProductID: productID, OrgID: orgID, ProductName: p.Name, SKU: p.SKU, TrackStock: false}, nil
	}
	_ = r.EnsureStockLevel(ctx, orgID, productID)
	var row models.StockLevelModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND product_id = ?", orgID, productID).Take(&row).Error; err != nil {
		return inventorydomain.StockLevel{}, err
	}
	return inventorydomain.StockLevel{
		ProductID:   row.ProductID,
		OrgID:       row.OrgID,
		ProductName: p.Name,
		SKU:         p.SKU,
		Quantity:    row.Quantity,
		MinQuantity: row.MinQuantity,
		TrackStock:  p.TrackStock,
		IsLowStock:  row.MinQuantity > 0 && row.Quantity <= row.MinQuantity,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}

func (r *Repository) ListLevels(ctx context.Context, p ListStockParams) ([]inventorydomain.StockLevel, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).
		Table("stock_levels sl").
		Joins("JOIN products p ON p.id = sl.product_id").
		Where("sl.org_id = ?", p.OrgID)
	if p.Archived {
		q = q.Where("p.deleted_at IS NOT NULL")
	} else {
		q = q.Where("p.deleted_at IS NULL")
	}
	if p.LowStock {
		q = q.Where("sl.min_quantity > 0 AND sl.quantity <= sl.min_quantity")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}
	order := "desc"
	if strings.EqualFold(p.Order, "asc") {
		order = "asc"
	}
	if p.After != nil && *p.After != uuid.Nil {
		if order == "asc" {
			q = q.Where("sl.product_id > ?", *p.After)
		} else {
			q = q.Where("sl.product_id < ?", *p.After)
		}
	}
	type row struct {
		ProductID   uuid.UUID `gorm:"column:product_id"`
		OrgID       uuid.UUID `gorm:"column:org_id"`
		ProductName string    `gorm:"column:product_name"`
		SKU         string    `gorm:"column:sku"`
		TrackStock  bool      `gorm:"column:track_stock"`
		Quantity    float64   `gorm:"column:quantity"`
		MinQuantity float64   `gorm:"column:min_quantity"`
		UpdatedAt   time.Time `gorm:"column:updated_at"`
	}
	var rows []row
	if err := q.Select("sl.product_id, sl.org_id, p.name as product_name, p.sku, p.track_stock, sl.quantity, sl.min_quantity, sl.updated_at").
		Order("sl.product_id " + order).Limit(limit + 1).Scan(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]inventorydomain.StockLevel, 0, len(rows))
	for _, row := range rows {
		out = append(out, inventorydomain.StockLevel{
			ProductID:   row.ProductID,
			OrgID:       row.OrgID,
			ProductName: row.ProductName,
			SKU:         row.SKU,
			TrackStock:  row.TrackStock,
			Quantity:    row.Quantity,
			MinQuantity: row.MinQuantity,
			IsLowStock:  row.MinQuantity > 0 && row.Quantity <= row.MinQuantity,
			UpdatedAt:   row.UpdatedAt,
		})
	}
	var next *uuid.UUID
	if hasMore && len(rows) > 0 {
		v := rows[len(rows)-1].ProductID
		next = &v
	}
	return out, total, hasMore, next, nil
}

func (r *Repository) ListMovements(ctx context.Context, p ListMovementParams) ([]inventorydomain.StockMovement, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).
		Table("stock_movements sm").
		Joins("JOIN products p ON p.id = sm.product_id").
		Where("sm.org_id = ?", p.OrgID)
	if p.ProductID != nil && *p.ProductID != uuid.Nil {
		q = q.Where("sm.product_id = ?", *p.ProductID)
	}
	if t := strings.TrimSpace(p.Type); t != "" {
		q = q.Where("sm.type = ?", t)
	}
	if p.After != nil && *p.After != uuid.Nil {
		q = q.Where("sm.id < ?", *p.After)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}
	type row struct {
		ID          uuid.UUID  `gorm:"column:id"`
		OrgID       uuid.UUID  `gorm:"column:org_id"`
		ProductID   uuid.UUID  `gorm:"column:product_id"`
		ProductName string     `gorm:"column:product_name"`
		Type        string     `gorm:"column:type"`
		Quantity    float64    `gorm:"column:quantity"`
		Reason      string     `gorm:"column:reason"`
		ReferenceID *uuid.UUID `gorm:"column:reference_id"`
		Notes       string     `gorm:"column:notes"`
		CreatedBy   string     `gorm:"column:created_by"`
		CreatedAt   time.Time  `gorm:"column:created_at"`
	}
	var rows []row
	if err := q.Select("sm.id, sm.org_id, sm.product_id, p.name as product_name, sm.type, sm.quantity, sm.reason, sm.reference_id, sm.notes, sm.created_by, sm.created_at").
		Order("sm.created_at DESC").Order("sm.id DESC").Limit(limit + 1).Scan(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]inventorydomain.StockMovement, 0, len(rows))
	for _, row := range rows {
		out = append(out, inventorydomain.StockMovement{
			ID:          row.ID,
			OrgID:       row.OrgID,
			ProductID:   row.ProductID,
			ProductName: row.ProductName,
			Type:        row.Type,
			Quantity:    row.Quantity,
			Reason:      row.Reason,
			ReferenceID: row.ReferenceID,
			Notes:       row.Notes,
			CreatedBy:   row.CreatedBy,
			CreatedAt:   row.CreatedAt,
		})
	}
	var next *uuid.UUID
	if hasMore && len(rows) > 0 {
		v := rows[len(rows)-1].ID
		next = &v
	}
	return out, total, hasMore, next, nil
}

func (r *Repository) AdjustAndMove(ctx context.Context, orgID, productID uuid.UUID, delta float64, reason string, referenceID *uuid.UUID, notes, actor string, minQuantity *float64, movementType string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		p, err := r.GetProduct(ctx, orgID, productID)
		if err != nil {
			return err
		}
		if !p.TrackStock {
			return nil
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "org_id"}, {Name: "product_id"}}, DoNothing: true}).
			Create(&models.StockLevelModel{OrgID: orgID, ProductID: productID, Quantity: 0, MinQuantity: 0, UpdatedAt: time.Now().UTC()}).Error; err != nil {
			return err
		}
		updates := map[string]any{"quantity": gorm.Expr("quantity + ?", delta), "updated_at": gorm.Expr("now()")}
		if minQuantity != nil {
			updates["min_quantity"] = *minQuantity
		}
		if err := tx.Model(&models.StockLevelModel{}).Where("org_id = ? AND product_id = ?", orgID, productID).Updates(updates).Error; err != nil {
			return err
		}
		mv := models.StockMovementModel{ID: uuid.New(), OrgID: orgID, ProductID: productID, Type: movementType, Quantity: delta, Reason: reason, ReferenceID: referenceID, Notes: notes, CreatedBy: actor, CreatedAt: time.Now().UTC()}
		if err := tx.Create(&mv).Error; err != nil {
			return err
		}
		return nil
	})
}
