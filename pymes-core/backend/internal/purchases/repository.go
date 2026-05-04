package purchases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	utils "github.com/devpablocristo/core/validate/go/stringutil"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/purchases/repository/models"
	purchasesdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/purchases/usecases/domain"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type tenantSettings struct {
	PurchasePrefix     string  `gorm:"column:purchase_prefix"`
	NextPurchaseNumber int     `gorm:"column:next_purchase_number"`
	Currency           string  `gorm:"column:currency"`
	TaxRate            float64 `gorm:"column:tax_rate"`
}

func normalizeBranchID(branchID *uuid.UUID) *uuid.UUID {
	if branchID == nil || *branchID == uuid.Nil {
		return nil
	}
	return branchID
}

func (r *Repository) List(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, status string, limit int) ([]purchasesdomain.Purchase, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&models.PurchaseModel{}).Where("org_id = ?", orgID)
	if branchID != nil && *branchID != uuid.Nil {
		q = q.Where("(branch_id = ? OR branch_id IS NULL)", *branchID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []models.PurchaseModel
	if err := q.Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]purchasesdomain.Purchase, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row, nil))
	}
	return out, nil
}

func (r *Repository) Create(ctx context.Context, in CreateInput) (purchasesdomain.Purchase, error) {
	var out purchasesdomain.Purchase
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tenant, err := r.getOrCreateTenantSettingsForUpdate(ctx, tx, in.OrgID)
		if err != nil {
			return err
		}
		number := fmt.Sprintf("%s-%05d", tenant.PurchasePrefix, tenant.NextPurchaseNumber)
		subtotal, taxTotal, total := totals(in.Items)
		purchaseRow := models.PurchaseModel{
			ID:            uuid.New(),
			OrgID:         in.OrgID,
			BranchID:      in.BranchID,
			Number:        number,
			SupplierID:    in.SupplierID,
			SupplierName:  in.SupplierName,
			Status:        in.Status,
			PaymentStatus: in.PaymentStatus,
			Subtotal:      subtotal,
			TaxTotal:      taxTotal,
			Total:         total,
			Currency:      tenant.Currency,
			Notes:         in.Notes,
			ReceivedAt:    markReceivedAt(in.Status),
			CreatedBy:     in.CreatedBy,
			CreatedAt:     time.Now().UTC(),
			UpdatedAt:     time.Now().UTC(),
			Tags:          pq.StringArray(utils.NormalizeTags(in.Tags)),
			Metadata:      metadataToJSONBytes(in.Metadata),
		}
		if err := tx.Create(&purchaseRow).Error; err != nil {
			return err
		}
		items := make([]models.PurchaseItemModel, 0, len(in.Items))
		for _, item := range in.Items {
			pid := item.ProductID
			id := item.ID
			if id == uuid.Nil {
				id = uuid.New()
			}
			items = append(items, models.PurchaseItemModel{ID: id, PurchaseID: purchaseRow.ID, ProductID: pid, ServiceID: item.ServiceID, Description: item.Description, Quantity: item.Quantity, UnitCost: item.UnitCost, TaxRate: item.TaxRate, Subtotal: item.Subtotal, SortOrder: item.SortOrder})
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}
		if err := tx.Table("tenant_settings").Where("org_id = ?", in.OrgID).Updates(map[string]any{"next_purchase_number": tenant.NextPurchaseNumber + 1, "updated_at": gorm.Expr("now()")}).Error; err != nil {
			return err
		}
		if in.Status == "received" {
			if err := r.applyStock(ctx, tx, in.OrgID, in.BranchID, purchaseRow.ID, items, in.CreatedBy); err != nil {
				return err
			}
		}
		out = toDomain(purchaseRow, items)
		return nil
	})
	if err != nil {
		return purchasesdomain.Purchase{}, err
	}
	return out, nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (purchasesdomain.Purchase, error) {
	return r.getByIDWithDB(ctx, r.db, orgID, id)
}

func (r *Repository) getByIDWithDB(ctx context.Context, db *gorm.DB, orgID, id uuid.UUID) (purchasesdomain.Purchase, error) {
	var row models.PurchaseModel
	if err := db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		return purchasesdomain.Purchase{}, err
	}
	var items []models.PurchaseItemModel
	if err := db.WithContext(ctx).Where("purchase_id = ?", id).Order("sort_order ASC").Find(&items).Error; err != nil {
		return purchasesdomain.Purchase{}, err
	}
	return toDomain(row, items), nil
}

func (r *Repository) Update(ctx context.Context, in UpdateInput) (purchasesdomain.Purchase, error) {
	var out purchasesdomain.Purchase
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current models.PurchaseModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("org_id = ? AND id = ?", in.OrgID, in.ID).Take(&current).Error; err != nil {
			return err
		}
		var currentItems []models.PurchaseItemModel
		if current.Status == "received" {
			if err := tx.Where("purchase_id = ?", in.ID).Order("sort_order ASC").Find(&currentItems).Error; err != nil {
				return err
			}
		}
		subtotal, taxTotal, total := totals(in.Items)
		updates := map[string]any{
			"party_id":        in.SupplierID,
			"party_name":      in.SupplierName,
			"branch_id":       in.BranchID,
			"status":          in.Status,
			"payment_status":  in.PaymentStatus,
			"subtotal":        subtotal,
			"tax_total":       taxTotal,
			"total":           total,
			"notes":           in.Notes,
			"tags":            pq.StringArray(utils.NormalizeTags(in.Tags)),
			"metadata":        metadataToJSONBytes(in.Metadata),
			"updated_at":      time.Now().UTC(),
		}
		if in.Status == "received" && current.ReceivedAt == nil {
			updates["received_at"] = time.Now().UTC()
		}
		if err := tx.Model(&models.PurchaseModel{}).Where("org_id = ? AND id = ?", in.OrgID, in.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Where("purchase_id = ?", in.ID).Delete(&models.PurchaseItemModel{}).Error; err != nil {
			return err
		}
		items := make([]models.PurchaseItemModel, 0, len(in.Items))
		for _, item := range in.Items {
			id := item.ID
			if id == uuid.Nil {
				id = uuid.New()
			}
			items = append(items, models.PurchaseItemModel{ID: id, PurchaseID: in.ID, ProductID: item.ProductID, ServiceID: item.ServiceID, Description: item.Description, Quantity: item.Quantity, UnitCost: item.UnitCost, TaxRate: item.TaxRate, Subtotal: item.Subtotal, SortOrder: item.SortOrder})
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}
		if current.Status == "received" {
			if err := r.reverseStock(ctx, tx, in.OrgID, current.BranchID, in.ID, currentItems, current.CreatedBy); err != nil {
				return err
			}
		}
		if in.Status == "received" {
			if err := r.applyStock(ctx, tx, in.OrgID, in.BranchID, in.ID, items, current.CreatedBy); err != nil {
				return err
			}
		}
		updated, err := r.getByIDWithDB(ctx, tx, in.OrgID, in.ID)
		if err != nil {
			return err
		}
		out = updated
		return nil
	})
	if err != nil {
		return purchasesdomain.Purchase{}, err
	}
	return out, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, in UpdateStatusInput) (purchasesdomain.Purchase, error) {
	var out purchasesdomain.Purchase
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current models.PurchaseModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("org_id = ? AND id = ?", in.OrgID, in.ID).Take(&current).Error; err != nil {
			return err
		}

		updates := map[string]any{
			"status":     in.Status,
			"updated_at": time.Now().UTC(),
		}
		if in.Status == "received" && current.ReceivedAt == nil {
			updates["received_at"] = time.Now().UTC()
		} else if in.Status != "received" && current.ReceivedAt != nil {
			updates["received_at"] = nil
		}
		if err := tx.Model(&models.PurchaseModel{}).Where("org_id = ? AND id = ?", in.OrgID, in.ID).Updates(updates).Error; err != nil {
			return err
		}

		if current.Status == in.Status {
			updated, err := r.getByIDWithDB(ctx, tx, in.OrgID, in.ID)
			if err != nil {
				return err
			}
			out = updated
			return nil
		}

		needsStockApply := current.Status != "received" && in.Status == "received"
		needsStockRevert := current.Status == "received" && in.Status != "received"
		if needsStockApply || needsStockRevert {
			var items []models.PurchaseItemModel
			if err := tx.Where("purchase_id = ?", in.ID).Order("sort_order ASC").Find(&items).Error; err != nil {
				return err
			}
			if needsStockRevert {
				if err := r.reverseStock(ctx, tx, in.OrgID, current.BranchID, in.ID, items, current.CreatedBy); err != nil {
					return err
				}
			}
			if needsStockApply {
				if err := r.applyStock(ctx, tx, in.OrgID, current.BranchID, in.ID, items, current.CreatedBy); err != nil {
					return err
				}
			}
		}

		updated, err := r.getByIDWithDB(ctx, tx, in.OrgID, in.ID)
		if err != nil {
			return err
		}
		out = updated
		return nil
	})
	if err != nil {
		return purchasesdomain.Purchase{}, err
	}
	return out, nil
}

// PatchAnnotations actualiza etiquetas, metadata y campos de texto permitidos fuera del borrador.
func (r *Repository) PatchAnnotations(ctx context.Context, orgID, id uuid.UUID, patch PurchasePatchFields) (purchasesdomain.Purchase, error) {
	var row models.PurchaseModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		return purchasesdomain.Purchase{}, err
	}
	updates := map[string]any{"updated_at": time.Now().UTC()}
	if patch.Tags != nil {
		updates["tags"] = pq.StringArray(utils.NormalizeTags(*patch.Tags))
	}
	if patch.Metadata != nil {
		merged, err := mergeMetadataJSON(row.Metadata, *patch.Metadata)
		if err != nil {
			return purchasesdomain.Purchase{}, err
		}
		updates["metadata"] = merged
	}
	if patch.Notes != nil {
		updates["notes"] = strings.TrimSpace(*patch.Notes)
	}
	if patch.PaymentStatus != nil {
		updates["payment_status"] = strings.TrimSpace(strings.ToLower(*patch.PaymentStatus))
	}
	if patch.SupplierName != nil {
		updates["party_name"] = strings.TrimSpace(*patch.SupplierName)
	}
	if len(updates) == 1 {
		return r.GetByID(ctx, orgID, id)
	}
	if err := r.db.WithContext(ctx).Model(&models.PurchaseModel{}).Where("org_id = ? AND id = ?", orgID, id).Updates(updates).Error; err != nil {
		return purchasesdomain.Purchase{}, err
	}
	return r.GetByID(ctx, orgID, id)
}

func (r *Repository) reverseStock(ctx context.Context, tx *gorm.DB, orgID uuid.UUID, branchID *uuid.UUID, purchaseID uuid.UUID, items []models.PurchaseItemModel, actor string) error {
	for _, item := range items {
		if item.ProductID == nil || *item.ProductID == uuid.Nil || item.Quantity <= 0 {
			continue
		}
		if err := r.adjustStockLevel(ctx, tx, orgID, branchID, *item.ProductID, -item.Quantity); err != nil {
			return err
		}
		if err := tx.Exec(`
			INSERT INTO stock_movements (id, org_id, branch_id, product_id, type, quantity, reason, reference_id, notes, created_by, created_at)
			VALUES (gen_random_uuid(), ?, ?, ?, 'out', ?, 'purchase_revert', ?, ?, ?, now())
		`, orgID, normalizeBranchID(branchID), *item.ProductID, item.Quantity, purchaseID, item.Description, actor).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) GetSupplierName(ctx context.Context, orgID, supplierID uuid.UUID) (string, error) {
	var name string
	err := r.db.WithContext(ctx).
		Table("parties p").
		Select("p.display_name").
		Joins("JOIN party_roles pr ON pr.party_id = p.id AND pr.org_id = p.org_id AND pr.role = 'supplier' AND pr.is_active = true").
		Where("p.org_id = ? AND p.id = ? AND p.deleted_at IS NULL", orgID, supplierID).
		Take(&name).Error
	return name, err
}

func (r *Repository) GetCurrency(ctx context.Context, orgID uuid.UUID) string {
	tenant, err := r.loadTenant(ctx, r.db, orgID)
	if err != nil {
		return "ARS"
	}
	return tenant.Currency
}
func (r *Repository) GetTaxRate(ctx context.Context, orgID uuid.UUID) float64 {
	tenant, err := r.loadTenant(ctx, r.db, orgID)
	if err != nil {
		return 21
	}
	return tenant.TaxRate
}

func (r *Repository) loadTenant(ctx context.Context, db *gorm.DB, orgID uuid.UUID) (tenantSettings, error) {
	var row tenantSettings
	err := db.WithContext(ctx).Table("tenant_settings").Select("purchase_prefix, next_purchase_number, currency, tax_rate").Where("org_id = ?", orgID).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tenantSettings{PurchasePrefix: "CPA", NextPurchaseNumber: 1, Currency: "ARS", TaxRate: 21}, nil
		}
		return tenantSettings{}, err
	}
	if strings.TrimSpace(row.PurchasePrefix) == "" {
		row.PurchasePrefix = "CPA"
	}
	if row.NextPurchaseNumber <= 0 {
		row.NextPurchaseNumber = 1
	}
	if strings.TrimSpace(row.Currency) == "" {
		row.Currency = "ARS"
	}
	if row.TaxRate <= 0 {
		row.TaxRate = 21
	}
	return row, nil
}

func (r *Repository) getOrCreateTenantSettingsForUpdate(ctx context.Context, tx *gorm.DB, orgID uuid.UUID) (tenantSettings, error) {
	row, err := r.loadTenant(ctx, tx.Clauses(clause.Locking{Strength: "UPDATE"}), orgID)
	if err == nil {
		return row, nil
	}
	if err := tx.WithContext(ctx).Exec(`INSERT INTO tenant_settings (org_id, plan_code, hard_limits, currency, tax_rate, purchase_prefix, next_purchase_number, created_at, updated_at) VALUES (?, 'starter', '{}'::jsonb, 'ARS', 21.0, 'CPA', 1, now(), now()) ON CONFLICT (org_id) DO NOTHING`, orgID).Error; err != nil {
		return tenantSettings{}, err
	}
	return r.loadTenant(ctx, tx.Clauses(clause.Locking{Strength: "UPDATE"}), orgID)
}

func (r *Repository) adjustStockLevel(ctx context.Context, tx *gorm.DB, orgID uuid.UUID, branchID *uuid.UUID, productID uuid.UUID, delta float64) error {
	normalizedBranchID := normalizeBranchID(branchID)
	now := time.Now().UTC()
	updates := map[string]any{
		"quantity":   gorm.Expr("quantity + ?", delta),
		"updated_at": now,
	}

	if normalizedBranchID != nil {
		res := tx.WithContext(ctx).
			Table("stock_levels").
			Where("org_id = ? AND product_id = ? AND branch_id = ?", orgID, productID, *normalizedBranchID).
			Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			return nil
		}
		res = tx.WithContext(ctx).
			Table("stock_levels").
			Where("org_id = ? AND product_id = ? AND branch_id IS NULL", orgID, productID).
			Updates(map[string]any{
				"branch_id":  *normalizedBranchID,
				"quantity":   gorm.Expr("quantity + ?", delta),
				"updated_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			return nil
		}
	}

	if normalizedBranchID == nil {
		res := tx.WithContext(ctx).
			Table("stock_levels").
			Where("org_id = ? AND product_id = ? AND branch_id IS NULL", orgID, productID).
			Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			return nil
		}
	}

	return tx.WithContext(ctx).Exec(
		`INSERT INTO stock_levels (product_id, org_id, branch_id, quantity, min_quantity, updated_at) VALUES (?, ?, ?, ?, 0, ?)`,
		productID,
		orgID,
		normalizedBranchID,
		delta,
		now,
	).Error
}

func (r *Repository) applyStock(ctx context.Context, tx *gorm.DB, orgID uuid.UUID, branchID *uuid.UUID, purchaseID uuid.UUID, items []models.PurchaseItemModel, actor string) error {
	for _, item := range items {
		if item.ProductID == nil || *item.ProductID == uuid.Nil || item.Quantity <= 0 {
			continue
		}
		if err := r.adjustStockLevel(ctx, tx, orgID, branchID, *item.ProductID, item.Quantity); err != nil {
			return err
		}
		if err := tx.Exec(`
			INSERT INTO stock_movements (id, org_id, branch_id, product_id, type, quantity, reason, reference_id, notes, created_by, created_at)
			VALUES (gen_random_uuid(), ?, ?, ?, 'in', ?, 'purchase', ?, ?, ?, now())
		`, orgID, normalizeBranchID(branchID), *item.ProductID, item.Quantity, purchaseID, item.Description, actor).Error; err != nil {
			return err
		}
	}
	return nil
}

func totals(items []purchasesdomain.PurchaseItem) (float64, float64, float64) {
	subtotal := 0.0
	taxTotal := 0.0
	for _, item := range items {
		line := item.Quantity * item.UnitCost
		subtotal += line
		taxTotal += line * item.TaxRate / 100
	}
	return subtotal, taxTotal, subtotal + taxTotal
}

func toDomain(row models.PurchaseModel, items []models.PurchaseItemModel) purchasesdomain.Purchase {
	out := purchasesdomain.Purchase{
		ID:            row.ID,
		OrgID:         row.OrgID,
		BranchID:      row.BranchID,
		Number:        row.Number,
		SupplierID:    row.SupplierID,
		SupplierName:  row.SupplierName,
		Status:        row.Status,
		PaymentStatus: row.PaymentStatus,
		Subtotal:      row.Subtotal,
		TaxTotal:      row.TaxTotal,
		Total:         row.Total,
		Currency:      row.Currency,
		Notes:         row.Notes,
		ReceivedAt:    row.ReceivedAt,
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
		Tags:          append([]string(nil), row.Tags...),
		Metadata:      metadataFromJSONBytes(row.Metadata),
	}
	for _, item := range items {
		out.Items = append(out.Items, purchasesdomain.PurchaseItem{ID: item.ID, PurchaseID: item.PurchaseID, ProductID: item.ProductID, ServiceID: item.ServiceID, Description: item.Description, Quantity: item.Quantity, UnitCost: item.UnitCost, TaxRate: item.TaxRate, Subtotal: item.Subtotal, SortOrder: item.SortOrder})
	}
	return out
}

func metadataFromJSONBytes(b []byte) map[string]any {
	if len(b) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		return map[string]any{}
	}
	return m
}

func metadataToJSONBytes(m map[string]any) []byte {
	if m == nil || len(m) == 0 {
		return []byte("{}")
	}
	b, err := json.Marshal(m)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func mergeMetadataJSON(current []byte, patch map[string]any) ([]byte, error) {
	base := metadataFromJSONBytes(current)
	for k, v := range patch {
		if k == "favorite" && !truthyMetadata(v) {
			delete(base, "favorite")
			continue
		}
		base[k] = v
	}
	return json.Marshal(base)
}

func truthyMetadata(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		return s == "true" || s == "1"
	case float64:
		return t != 0
	case int:
		return t != 0
	default:
		return v != nil
	}
}
