package purchases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/purchases/repository/models"
	purchasesdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/purchases/usecases/domain"
	"github.com/devpablocristo/pymes/pkgs/go-pkg/pagination"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type tenantSettings struct {
	PurchasePrefix     string  `gorm:"column:purchase_prefix"`
	NextPurchaseNumber int     `gorm:"column:next_purchase_number"`
	Currency           string  `gorm:"column:currency"`
	TaxRate            float64 `gorm:"column:tax_rate"`
}

func (r *Repository) List(ctx context.Context, orgID uuid.UUID, status string, limit int) ([]purchasesdomain.Purchase, error) {
	limit = pagination.NormalizeLimit(limit, 20, 100)
	q := r.db.WithContext(ctx).Model(&models.PurchaseModel{}).Where("org_id = ?", orgID)
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
		purchaseRow := models.PurchaseModel{ID: uuid.New(), OrgID: in.OrgID, Number: number, SupplierID: in.SupplierID, SupplierName: in.SupplierName, Status: in.Status, PaymentStatus: in.PaymentStatus, Subtotal: subtotal, TaxTotal: taxTotal, Total: total, Currency: tenant.Currency, Notes: in.Notes, ReceivedAt: markReceivedAt(in.Status), CreatedBy: in.CreatedBy, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
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
			items = append(items, models.PurchaseItemModel{ID: id, PurchaseID: purchaseRow.ID, ProductID: pid, Description: item.Description, Quantity: item.Quantity, UnitCost: item.UnitCost, TaxRate: item.TaxRate, Subtotal: item.Subtotal, SortOrder: item.SortOrder})
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
			if err := r.applyStock(ctx, tx, in.OrgID, purchaseRow.ID, items, in.CreatedBy); err != nil {
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
	var row models.PurchaseModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		return purchasesdomain.Purchase{}, err
	}
	var items []models.PurchaseItemModel
	if err := r.db.WithContext(ctx).Where("purchase_id = ?", id).Order("sort_order ASC").Find(&items).Error; err != nil {
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
		subtotal, taxTotal, total := totals(in.Items)
		updates := map[string]any{"party_id": in.SupplierID, "party_name": in.SupplierName, "status": in.Status, "payment_status": in.PaymentStatus, "subtotal": subtotal, "tax_total": taxTotal, "total": total, "notes": in.Notes, "updated_at": time.Now().UTC()}
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
			items = append(items, models.PurchaseItemModel{ID: id, PurchaseID: in.ID, ProductID: item.ProductID, Description: item.Description, Quantity: item.Quantity, UnitCost: item.UnitCost, TaxRate: item.TaxRate, Subtotal: item.Subtotal, SortOrder: item.SortOrder})
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}
		if current.Status != "received" && in.Status == "received" {
			if err := r.applyStock(ctx, tx, in.OrgID, in.ID, items, current.CreatedBy); err != nil {
				return err
			}
		}
		updated, err := r.GetByID(ctx, in.OrgID, in.ID)
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

func (r *Repository) GetSupplierName(ctx context.Context, orgID, supplierID uuid.UUID) (string, error) {
	var name string
	err := r.db.WithContext(ctx).Table("suppliers").Select("name").Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, supplierID).Take(&name).Error
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

func (r *Repository) applyStock(ctx context.Context, tx *gorm.DB, orgID, purchaseID uuid.UUID, items []models.PurchaseItemModel, actor string) error {
	for _, item := range items {
		if item.ProductID == nil || *item.ProductID == uuid.Nil || item.Quantity <= 0 {
			continue
		}
		if err := tx.Exec(`
			INSERT INTO stock_levels (product_id, org_id, quantity, min_quantity, updated_at)
			VALUES (?, ?, ?, 0, now())
			ON CONFLICT (org_id, product_id)
			DO UPDATE SET quantity = stock_levels.quantity + EXCLUDED.quantity, updated_at = now()
		`, *item.ProductID, orgID, item.Quantity).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			INSERT INTO stock_movements (id, org_id, product_id, type, quantity, reason, reference_id, notes, created_by, created_at)
			VALUES (gen_random_uuid(), ?, ?, 'in', ?, 'purchase', ?, ?, ?, now())
		`, orgID, *item.ProductID, item.Quantity, purchaseID, item.Description, actor).Error; err != nil {
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
	out := purchasesdomain.Purchase{ID: row.ID, OrgID: row.OrgID, Number: row.Number, SupplierID: row.SupplierID, SupplierName: row.SupplierName, Status: row.Status, PaymentStatus: row.PaymentStatus, Subtotal: row.Subtotal, TaxTotal: row.TaxTotal, Total: row.Total, Currency: row.Currency, Notes: row.Notes, ReceivedAt: row.ReceivedAt, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	for _, item := range items {
		out.Items = append(out.Items, purchasesdomain.PurchaseItem{ID: item.ID, PurchaseID: item.PurchaseID, ProductID: item.ProductID, Description: item.Description, Quantity: item.Quantity, UnitCost: item.UnitCost, TaxRate: item.TaxRate, Subtotal: item.Subtotal, SortOrder: item.SortOrder})
	}
	return out
}
