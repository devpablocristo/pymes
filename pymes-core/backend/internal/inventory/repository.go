package inventory

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/inventory/repository/models"
	inventorydomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/inventory/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type ListStockParams struct {
	TenantID uuid.UUID
	BranchID *uuid.UUID
	Limit    int
	After    *uuid.UUID
	LowStock bool
	Archived bool
	Order    string
}

type ListMovementParams struct {
	TenantID  uuid.UUID
	BranchID  *uuid.UUID
	Limit     int
	After     *uuid.UUID
	ProductID *uuid.UUID
	Type      string
}

type productRow struct {
	ID         uuid.UUID
	Name       string
	SKU        string
	TrackStock bool      `gorm:"column:track_stock"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func normalizeBranchID(branchID *uuid.UUID) *uuid.UUID {
	if branchID == nil || *branchID == uuid.Nil {
		return nil
	}
	return branchID
}

func stockLevelSelectBranchExpr(branchID *uuid.UUID) string {
	if normalizeBranchID(branchID) != nil {
		return "sl.branch_id AS branch_id"
	}
	return "NULL AS branch_id"
}

func (r *Repository) EnsureStockLevel(ctx context.Context, tenantID, productID uuid.UUID) error {
	_ = ctx
	_ = tenantID
	_ = productID
	// El stock ya no se materializa al crear el producto: se resuelve por branch cuando hay
	// movimientos o ajustes, y los listados sintetizan 0 si no existe fila todavía.
	return nil
}

func (r *Repository) GetProduct(ctx context.Context, tenantID, productID uuid.UUID) (productRow, error) {
	var row productRow
	err := r.db.WithContext(ctx).
		Table("products").
		Select("id, name, sku, track_stock, updated_at").
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, productID).
		Take(&row).Error
	if err != nil {
		return productRow{}, err
	}
	return row, nil
}

func (r *Repository) getPreferredLevel(ctx context.Context, tenantID uuid.UUID, branchID *uuid.UUID, productID uuid.UUID) (models.StockLevelModel, bool, error) {
	normalizedBranchID := normalizeBranchID(branchID)
	if normalizedBranchID == nil {
		type aggregateRow struct {
			Quantity    float64    `gorm:"column:quantity"`
			MinQuantity float64    `gorm:"column:min_quantity"`
			UpdatedAt   *time.Time `gorm:"column:updated_at"`
			RowCount    int64      `gorm:"column:row_count"`
		}
		var row aggregateRow
		if err := r.db.WithContext(ctx).Raw(`
			SELECT
				COUNT(*) AS row_count,
				COALESCE(SUM(quantity), 0) AS quantity,
				COALESCE(MAX(min_quantity), 0) AS min_quantity,
				MAX(updated_at) AS updated_at
			FROM stock_levels
			WHERE tenant_id = ? AND product_id = ?
		`, tenantID, productID).Scan(&row).Error; err != nil {
			return models.StockLevelModel{}, false, err
		}
		if row.RowCount == 0 {
			return models.StockLevelModel{}, false, nil
		}
		level := models.StockLevelModel{
			TenantID:    tenantID,
			ProductID:   productID,
			Quantity:    row.Quantity,
			MinQuantity: row.MinQuantity,
		}
		if row.UpdatedAt != nil {
			level.UpdatedAt = *row.UpdatedAt
		}
		return level, true, nil
	}

	var row models.StockLevelModel
	result := r.db.WithContext(ctx).Raw(`
		SELECT product_id, tenant_id, branch_id, quantity, min_quantity, updated_at
		FROM stock_levels
		WHERE tenant_id = ? AND product_id = ? AND (branch_id = ? OR branch_id IS NULL)
		ORDER BY CASE WHEN branch_id = ? THEN 0 ELSE 1 END, updated_at DESC
		LIMIT 1
	`, tenantID, productID, *normalizedBranchID, *normalizedBranchID).Scan(&row)
	if result.Error != nil {
		return models.StockLevelModel{}, false, result.Error
	}
	if result.RowsAffected == 0 {
		return models.StockLevelModel{}, false, nil
	}
	return row, true, nil
}

func (r *Repository) GetLevel(ctx context.Context, tenantID uuid.UUID, branchID *uuid.UUID, productID uuid.UUID) (inventorydomain.StockLevel, error) {
	p, err := r.GetProduct(ctx, tenantID, productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return inventorydomain.StockLevel{}, gorm.ErrRecordNotFound
		}
		return inventorydomain.StockLevel{}, err
	}
	normalizedBranchID := normalizeBranchID(branchID)
	if !p.TrackStock {
		return inventorydomain.StockLevel{
			ProductID:   productID,
			TenantID:    tenantID,
			BranchID:    normalizedBranchID,
			ProductName: p.Name,
			SKU:         p.SKU,
			TrackStock:  false,
		}, nil
	}

	row, found, err := r.getPreferredLevel(ctx, tenantID, normalizedBranchID, productID)
	if err != nil {
		return inventorydomain.StockLevel{}, err
	}
	if !found {
		return inventorydomain.StockLevel{
			ProductID:   productID,
			TenantID:    tenantID,
			BranchID:    normalizedBranchID,
			ProductName: p.Name,
			SKU:         p.SKU,
			Quantity:    0,
			MinQuantity: 0,
			TrackStock:  true,
			UpdatedAt:   p.UpdatedAt,
		}, nil
	}
	if normalizedBranchID != nil && row.BranchID == nil {
		row.BranchID = normalizedBranchID
	}
	return inventorydomain.StockLevel{
		ProductID:   row.ProductID,
		TenantID:    row.TenantID,
		BranchID:    row.BranchID,
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
	order := "desc"
	if strings.EqualFold(p.Order, "asc") {
		order = "asc"
	}
	normalizedBranchID := normalizeBranchID(p.BranchID)

	q := r.db.WithContext(ctx).
		Table("products p").
		Where("p.tenant_id = ? AND p.track_stock = true", p.TenantID)
	if p.Archived {
		q = q.Where("p.deleted_at IS NOT NULL")
	} else {
		q = q.Where("p.deleted_at IS NULL")
	}
	if p.After != nil && *p.After != uuid.Nil {
		if order == "asc" {
			q = q.Where("p.id > ?", *p.After)
		} else {
			q = q.Where("p.id < ?", *p.After)
		}
	}

	if normalizedBranchID == nil {
		q = q.Joins(`
			LEFT JOIN (
				SELECT
					tenant_id,
					product_id,
					SUM(quantity) AS quantity,
					MAX(min_quantity) AS min_quantity,
					MAX(updated_at) AS updated_at
				FROM stock_levels
				WHERE tenant_id = ?
				GROUP BY tenant_id, product_id
			) sl ON sl.tenant_id = p.tenant_id AND sl.product_id = p.id
		`, p.TenantID)
		if p.LowStock {
			q = q.Where("COALESCE(sl.min_quantity, 0) > 0 AND COALESCE(sl.quantity, 0) <= COALESCE(sl.min_quantity, 0)")
		}
	} else {
		q = q.Joins(`
			LEFT JOIN LATERAL (
				SELECT
					product_id,
					tenant_id,
					branch_id,
					quantity,
					min_quantity,
					updated_at
				FROM stock_levels sl
				WHERE sl.tenant_id = p.tenant_id
				  AND sl.product_id = p.id
				  AND (sl.branch_id = ? OR sl.branch_id IS NULL)
				ORDER BY CASE WHEN sl.branch_id = ? THEN 0 ELSE 1 END, sl.updated_at DESC
				LIMIT 1
			) sl ON true
		`, *normalizedBranchID, *normalizedBranchID)
		if p.LowStock {
			q = q.Where("COALESCE(sl.min_quantity, 0) > 0 AND COALESCE(sl.quantity, 0) <= COALESCE(sl.min_quantity, 0)")
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}

	type row struct {
		ProductID   uuid.UUID  `gorm:"column:product_id"`
		TenantID    uuid.UUID  `gorm:"column:tenant_id"`
		BranchID    *uuid.UUID `gorm:"column:branch_id"`
		ProductName string     `gorm:"column:product_name"`
		SKU         string     `gorm:"column:sku"`
		TrackStock  bool       `gorm:"column:track_stock"`
		Quantity    float64    `gorm:"column:quantity"`
		MinQuantity float64    `gorm:"column:min_quantity"`
		UpdatedAt   time.Time  `gorm:"column:updated_at"`
	}

	var rows []row
	selectBranchID := stockLevelSelectBranchExpr(normalizedBranchID)
	if err := q.Select(`
			p.id AS product_id,
			p.tenant_id AS tenant_id,
			` + selectBranchID + `,
			p.name AS product_name,
			p.sku AS sku,
			p.track_stock AS track_stock,
			COALESCE(sl.quantity, 0) AS quantity,
			COALESCE(sl.min_quantity, 0) AS min_quantity,
			COALESCE(sl.updated_at, p.updated_at) AS updated_at
		`).
		Order("p.id " + order).
		Limit(limit + 1).
		Scan(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	out := make([]inventorydomain.StockLevel, 0, len(rows))
	for _, row := range rows {
		branchID := row.BranchID
		if normalizedBranchID != nil && branchID == nil {
			branchID = normalizedBranchID
		}
		out = append(out, inventorydomain.StockLevel{
			ProductID:   row.ProductID,
			TenantID:    row.TenantID,
			BranchID:    branchID,
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
	normalizedBranchID := normalizeBranchID(p.BranchID)

	q := r.db.WithContext(ctx).
		Table("stock_movements sm").
		Joins("JOIN products p ON p.id = sm.product_id").
		Where("sm.tenant_id = ?", p.TenantID)
	if normalizedBranchID != nil {
		q = q.Where("(sm.branch_id = ? OR sm.branch_id IS NULL)", *normalizedBranchID)
	}
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
		TenantID    uuid.UUID  `gorm:"column:tenant_id"`
		BranchID    *uuid.UUID `gorm:"column:branch_id"`
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
	if err := q.Select("sm.id, sm.tenant_id, sm.branch_id, sm.product_id, p.name as product_name, sm.type, sm.quantity, sm.reason, sm.reference_id, sm.notes, sm.created_by, sm.created_at").
		Order("sm.created_at DESC").Order("sm.id DESC").Limit(limit + 1).Scan(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	out := make([]inventorydomain.StockMovement, 0, len(rows))
	for _, row := range rows {
		branchID := row.BranchID
		if normalizedBranchID != nil && branchID == nil {
			branchID = normalizedBranchID
		}
		out = append(out, inventorydomain.StockMovement{
			ID:          row.ID,
			TenantID:    row.TenantID,
			BranchID:    branchID,
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

func (r *Repository) adjustStockLevel(ctx context.Context, tx *gorm.DB, tenantID uuid.UUID, branchID *uuid.UUID, productID uuid.UUID, delta float64, minQuantity *float64) error {
	normalizedBranchID := normalizeBranchID(branchID)
	now := time.Now().UTC()

	makeUpdates := func() map[string]any {
		updates := map[string]any{
			"quantity":   gorm.Expr("quantity + ?", delta),
			"updated_at": now,
		}
		if minQuantity != nil {
			updates["min_quantity"] = *minQuantity
		}
		return updates
	}

	if normalizedBranchID != nil {
		res := tx.WithContext(ctx).
			Model(&models.StockLevelModel{}).
			Where("tenant_id = ? AND product_id = ? AND branch_id = ?", tenantID, productID, *normalizedBranchID).
			Updates(makeUpdates())
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			return nil
		}

		promoteUpdates := makeUpdates()
		promoteUpdates["branch_id"] = *normalizedBranchID
		res = tx.WithContext(ctx).
			Model(&models.StockLevelModel{}).
			Where("tenant_id = ? AND product_id = ? AND branch_id IS NULL", tenantID, productID).
			Updates(promoteUpdates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			return nil
		}
	}

	if normalizedBranchID == nil {
		res := tx.WithContext(ctx).
			Model(&models.StockLevelModel{}).
			Where("tenant_id = ? AND product_id = ? AND branch_id IS NULL", tenantID, productID).
			Updates(makeUpdates())
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			return nil
		}
	}

	row := models.StockLevelModel{
		TenantID:    tenantID,
		BranchID:    normalizedBranchID,
		ProductID:   productID,
		Quantity:    delta,
		MinQuantity: 0,
		UpdatedAt:   now,
	}
	if minQuantity != nil {
		row.MinQuantity = *minQuantity
	}
	return tx.WithContext(ctx).Create(&row).Error
}

func (r *Repository) AdjustAndMove(ctx context.Context, tenantID uuid.UUID, branchID *uuid.UUID, productID uuid.UUID, delta float64, reason string, referenceID *uuid.UUID, notes, actor string, minQuantity *float64, movementType string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		p, err := r.GetProduct(ctx, tenantID, productID)
		if err != nil {
			return err
		}
		if !p.TrackStock {
			return nil
		}
		if err := r.adjustStockLevel(ctx, tx, tenantID, branchID, productID, delta, minQuantity); err != nil {
			return err
		}
		mv := models.StockMovementModel{
			ID:          uuid.New(),
			TenantID:    tenantID,
			BranchID:    normalizeBranchID(branchID),
			ProductID:   productID,
			Type:        movementType,
			Quantity:    delta,
			Reason:      reason,
			ReferenceID: referenceID,
			Notes:       notes,
			CreatedBy:   actor,
			CreatedAt:   time.Now().UTC(),
		}
		return tx.WithContext(ctx).Create(&mv).Error
	})
}
