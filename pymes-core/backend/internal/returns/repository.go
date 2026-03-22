// Package returns provides persistence for returns and credit notes.
package returns

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/devpablocristo/core/backend/go/apperror"
	"github.com/devpablocristo/core/backend/go/pagination"
	returnmodels "github.com/devpablocristo/pymes/pymes-core/backend/internal/returns/repository/models"
	returndomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/returns/usecases/domain"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type CreateReturnItemInput struct {
	SaleItemID uuid.UUID
	Quantity   float64
}

type CreateReturnInput struct {
	OrgID        uuid.UUID
	SaleID       uuid.UUID
	Reason       string
	RefundMethod string
	Notes        string
	CreatedBy    string
	Items        []CreateReturnItemInput
}

type ApplyCreditInput struct {
	OrgID        uuid.UUID
	SaleID       uuid.UUID
	CreditNoteID uuid.UUID
	Amount       float64
	Actor        string
}

func (r *Repository) List(ctx context.Context, orgID uuid.UUID, limit int) ([]returndomain.Return, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	salePartyIDExpr, err := r.salesPartyIDSelectExpr(ctx, "s")
	if err != nil {
		return nil, err
	}
	salePartyNameExpr, err := r.salesPartyNameSelectExpr(ctx, "s")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		returnmodels.ReturnModel
		PartyID   *uuid.UUID `gorm:"column:party_id"`
		PartyName string     `gorm:"column:party_name"`
	}
	err = r.db.WithContext(ctx).Table("returns r").
		Select(fmt.Sprintf("r.*, %s, %s", salePartyIDExpr, salePartyNameExpr)).
		Joins("JOIN sales s ON s.id = r.sale_id").
		Where("r.org_id = ?", orgID).
		Order("r.created_at DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]returndomain.Return, 0, len(rows))
	for _, row := range rows {
		out = append(out, toReturnDomain(row.ReturnModel, nil, row.PartyID, row.PartyName))
	}
	return out, nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (returndomain.Return, error) {
	salePartyIDExpr, err := r.salesPartyIDSelectExpr(ctx, "s")
	if err != nil {
		return returndomain.Return{}, err
	}
	salePartyNameExpr, err := r.salesPartyNameSelectExpr(ctx, "s")
	if err != nil {
		return returndomain.Return{}, err
	}
	var row struct {
		returnmodels.ReturnModel
		PartyID   *uuid.UUID `gorm:"column:party_id"`
		PartyName string     `gorm:"column:party_name"`
	}
	err = r.db.WithContext(ctx).Table("returns r").
		Select(fmt.Sprintf("r.*, %s, %s", salePartyIDExpr, salePartyNameExpr)).
		Joins("JOIN sales s ON s.id = r.sale_id").
		Where("r.org_id = ? AND r.id = ?", orgID, id).
		Take(&row).Error
	if err != nil {
		return returndomain.Return{}, err
	}
	var items []returnmodels.ReturnItemModel
	if err := r.db.WithContext(ctx).Where("return_id = ?", id).Find(&items).Error; err != nil {
		return returndomain.Return{}, err
	}
	return toReturnDomain(row.ReturnModel, items, row.PartyID, row.PartyName), nil
}

func (r *Repository) Create(ctx context.Context, in CreateReturnInput) (returndomain.Return, *returndomain.CreditNote, error) {
	var out returndomain.Return
	var credit *returndomain.CreditNote
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		salePartyIDExpr, err := r.salesPartyIDSelectExpr(ctx, "")
		if err != nil {
			return err
		}
		salePartyNameExpr, err := r.salesPartyNameSelectExpr(ctx, "")
		if err != nil {
			return err
		}
		var sale struct {
			Number        string
			PartyID       *uuid.UUID `gorm:"column:party_id"`
			PartyName     string     `gorm:"column:party_name"`
			AmountPaid    float64    `gorm:"column:amount_paid"`
			Total         float64
			Currency      string
			PaymentMethod string `gorm:"column:payment_method"`
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("sales").Select(fmt.Sprintf("number, %s, %s, amount_paid, total, currency, payment_method", salePartyIDExpr, salePartyNameExpr)).Where("org_id = ? AND id = ?", in.OrgID, in.SaleID).Take(&sale).Error; err != nil {
			return err
		}
		if len(in.Items) == 0 {
			return apperror.NewBadInput("items are required")
		}

		var settings struct {
			ReturnPrefix string `gorm:"column:return_prefix"`
			NextReturn   int    `gorm:"column:next_return_number"`
			CreditPrefix string `gorm:"column:credit_note_prefix"`
			NextCredit   int    `gorm:"column:next_credit_note_number"`
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("tenant_settings").Select("return_prefix, next_return_number, credit_note_prefix, next_credit_note_number").Where("org_id = ?", in.OrgID).Take(&settings).Error; err != nil {
			return err
		}

		returnNumber := fmt.Sprintf("%s-%05d", defaultString(settings.ReturnPrefix, "DEV"), maxInt(settings.NextReturn, 1))
		if err := tx.Exec("UPDATE tenant_settings SET next_return_number = ? WHERE org_id = ?", maxInt(settings.NextReturn, 1)+1, in.OrgID).Error; err != nil {
			return err
		}

		returnID := uuid.New()
		subtotal, taxTotal := 0.0, 0.0
		itemModels := make([]returnmodels.ReturnItemModel, 0, len(in.Items))
		for _, item := range in.Items {
			var saleItem struct {
				ID          uuid.UUID
				ProductID   *uuid.UUID `gorm:"column:product_id"`
				Description string
				Quantity    float64
				UnitPrice   float64 `gorm:"column:unit_price"`
				TaxRate     float64 `gorm:"column:tax_rate"`
				TrackStock  bool    `gorm:"column:track_stock"`
			}
			if err := tx.Table("sale_items si").Select("si.id, si.product_id, si.description, si.quantity, si.unit_price, si.tax_rate, COALESCE(p.track_stock, false) as track_stock").Joins("LEFT JOIN products p ON p.id = si.product_id").Where("si.sale_id = ? AND si.id = ?", in.SaleID, item.SaleItemID).Take(&saleItem).Error; err != nil {
				return err
			}
			var alreadyReturned float64
			if err := tx.Table("return_items ri").Select("COALESCE(SUM(ri.quantity),0)").Joins("JOIN returns r ON r.id = ri.return_id").Where("r.sale_id = ? AND ri.sale_item_id = ? AND r.status <> 'voided'", in.SaleID, item.SaleItemID).Take(&alreadyReturned).Error; err != nil {
				return err
			}
			available := saleItem.Quantity - alreadyReturned
			if item.Quantity <= 0 || item.Quantity > available {
				return apperror.NewBusinessRule(fmt.Sprintf("return quantity exceeds sold quantity for item %s", saleItem.Description))
			}
			lineSubtotal := item.Quantity * saleItem.UnitPrice
			subtotal += lineSubtotal
			taxTotal += lineSubtotal * saleItem.TaxRate / 100.0
			itemModels = append(itemModels, returnmodels.ReturnItemModel{ID: uuid.New(), ReturnID: returnID, SaleItemID: saleItem.ID, ProductID: saleItem.ProductID, Description: saleItem.Description, Quantity: item.Quantity, UnitPrice: saleItem.UnitPrice, TaxRate: saleItem.TaxRate, Subtotal: lineSubtotal})
		}
		total := subtotal + taxTotal
		returnRow := returnmodels.ReturnModel{ID: returnID, OrgID: in.OrgID, Number: returnNumber, SaleID: in.SaleID, Reason: in.Reason, Subtotal: subtotal, TaxTotal: taxTotal, Total: total, RefundMethod: in.RefundMethod, Status: "completed", Notes: in.Notes, CreatedBy: in.CreatedBy, CreatedAt: time.Now().UTC()}
		if err := tx.Create(&returnRow).Error; err != nil {
			return err
		}
		if err := tx.Create(&itemModels).Error; err != nil {
			return err
		}

		if in.RefundMethod == "credit_note" && sale.PartyID != nil && *sale.PartyID != uuid.Nil {
			creditNumber := fmt.Sprintf("%s-%05d", defaultString(settings.CreditPrefix, "NC"), maxInt(settings.NextCredit, 1))
			if err := tx.Exec("UPDATE tenant_settings SET next_credit_note_number = ? WHERE org_id = ?", maxInt(settings.NextCredit, 1)+1, in.OrgID).Error; err != nil {
				return err
			}
			creditRow := returnmodels.CreditNoteModel{ID: uuid.New(), OrgID: in.OrgID, Number: creditNumber, PartyID: *sale.PartyID, ReturnID: returnID, Amount: total, UsedAmount: 0, Balance: total, Status: "active", CreatedAt: time.Now().UTC()}
			if err := tx.Create(&creditRow).Error; err != nil {
				return err
			}
			creditDomain := toCreditDomain(creditRow)
			credit = &creditDomain
		} else {
			method := in.RefundMethod
			if method == "original_method" {
				method = defaultString(sale.PaymentMethod, "other")
			}
			if err := tx.Exec(`INSERT INTO cash_movements (id, org_id, type, amount, currency, category, description, payment_method, reference_type, reference_id, created_by, created_at) VALUES (gen_random_uuid(), ?, 'expense', ?, ?, 'return', ?, ?, 'return', ?, ?, now())`, in.OrgID, total, defaultString(sale.Currency, "ARS"), defaultString(in.Notes, "sale return refund"), method, returnID, in.CreatedBy).Error; err != nil {
				return err
			}
			newAmountPaid := sale.AmountPaid - total
			if newAmountPaid < 0 {
				newAmountPaid = 0
			}
			var returnedTotal float64
			if err := tx.Table("returns").Select("COALESCE(SUM(total),0)").Where("org_id = ? AND sale_id = ? AND status <> 'voided'", in.OrgID, in.SaleID).Take(&returnedTotal).Error; err != nil {
				return err
			}
			effectiveTotal := sale.Total - returnedTotal
			status := paymentStatus(newAmountPaid, effectiveTotal)
			if err := tx.Exec("UPDATE sales SET amount_paid = ?, payment_status = ? WHERE org_id = ? AND id = ?", newAmountPaid, status, in.OrgID, in.SaleID).Error; err != nil {
				return err
			}
		}

		for _, item := range itemModels {
			if item.ProductID == nil || *item.ProductID == uuid.Nil {
				continue
			}
			if err := tx.Exec(`INSERT INTO stock_levels (product_id, org_id, quantity, min_quantity, updated_at) VALUES (?, ?, 0, 0, now()) ON CONFLICT (product_id, org_id) DO NOTHING`, *item.ProductID, in.OrgID).Error; err != nil {
				return err
			}
			if err := tx.Exec(`UPDATE stock_levels SET quantity = quantity + ?, updated_at = now() WHERE product_id = ? AND org_id = ?`, item.Quantity, *item.ProductID, in.OrgID).Error; err != nil {
				return err
			}
			if err := tx.Exec(`INSERT INTO stock_movements (id, org_id, product_id, type, quantity, reason, reference_id, notes, created_by, created_at) VALUES (gen_random_uuid(), ?, ?, 'in', ?, 'return', ?, ?, ?, now())`, in.OrgID, *item.ProductID, item.Quantity, returnID, "sale return restock", in.CreatedBy).Error; err != nil {
				return err
			}
		}

		out = toReturnDomain(returnRow, itemModels, sale.PartyID, sale.PartyName)
		return nil
	})
	if err != nil {
		return returndomain.Return{}, nil, err
	}
	return out, credit, nil
}

func (r *Repository) Void(ctx context.Context, orgID, id uuid.UUID, actor string) (returndomain.Return, error) {
	var out returndomain.Return
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		item, err := r.GetByID(ctx, orgID, id)
		if err != nil {
			return err
		}
		if item.Status == "voided" {
			out = item
			return nil
		}
		var sale struct {
			AmountPaid    float64 `gorm:"column:amount_paid"`
			Total         float64
			Currency      string
			PaymentMethod string `gorm:"column:payment_method"`
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("sales").Select("amount_paid, total, currency, payment_method").Where("org_id = ? AND id = ?", orgID, item.SaleID).Take(&sale).Error; err != nil {
			return err
		}
		if err := tx.Model(&returnmodels.ReturnModel{}).Where("org_id = ? AND id = ?", orgID, id).Update("status", "voided").Error; err != nil {
			return err
		}
		if item.RefundMethod == "credit_note" {
			if err := tx.Model(&returnmodels.CreditNoteModel{}).Where("org_id = ? AND return_id = ? AND status = 'active'", orgID, id).Updates(map[string]any{"status": "voided", "balance": 0}).Error; err != nil {
				return err
			}
		} else {
			method := item.RefundMethod
			if method == "original_method" {
				method = defaultString(sale.PaymentMethod, "other")
			}
			if err := tx.Exec(`INSERT INTO cash_movements (id, org_id, type, amount, currency, category, description, payment_method, reference_type, reference_id, created_by, created_at) VALUES (gen_random_uuid(), ?, 'income', ?, ?, 'return', ?, ?, 'return', ?, ?, now())`, orgID, item.Total, defaultString(sale.Currency, "ARS"), "return void reversal", method, id, actor).Error; err != nil {
				return err
			}
			newAmountPaid := sale.AmountPaid + item.Total
			var returnedTotal float64
			if err := tx.Table("returns").Select("COALESCE(SUM(total),0)").Where("org_id = ? AND sale_id = ? AND status <> 'voided'", orgID, item.SaleID).Take(&returnedTotal).Error; err != nil {
				return err
			}
			effectiveTotal := sale.Total - returnedTotal
			status := paymentStatus(newAmountPaid, effectiveTotal)
			if err := tx.Exec("UPDATE sales SET amount_paid = ?, payment_status = ? WHERE org_id = ? AND id = ?", newAmountPaid, status, orgID, item.SaleID).Error; err != nil {
				return err
			}
		}
		for _, ri := range item.Items {
			if ri.ProductID == nil || *ri.ProductID == uuid.Nil {
				continue
			}
			if err := tx.Exec(`UPDATE stock_levels SET quantity = quantity - ?, updated_at = now() WHERE product_id = ? AND org_id = ?`, ri.Quantity, *ri.ProductID, orgID).Error; err != nil {
				return err
			}
			if err := tx.Exec(`INSERT INTO stock_movements (id, org_id, product_id, type, quantity, reason, reference_id, notes, created_by, created_at) VALUES (gen_random_uuid(), ?, ?, 'out', ?, 'return_void', ?, ?, ?, now())`, orgID, *ri.ProductID, ri.Quantity, id, "return void stock reversal", actor).Error; err != nil {
				return err
			}
		}
		out, err = r.GetByID(ctx, orgID, id)
		return err
	})
	if err != nil {
		return returndomain.Return{}, err
	}
	return out, nil
}

func (r *Repository) ListCreditNotes(ctx context.Context, orgID uuid.UUID, partyID *uuid.UUID, limit int) ([]returndomain.CreditNote, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&returnmodels.CreditNoteModel{}).Where("org_id = ?", orgID)
	if partyID != nil && *partyID != uuid.Nil {
		q = q.Where("party_id = ?", *partyID)
	}
	var rows []returnmodels.CreditNoteModel
	if err := q.Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]returndomain.CreditNote, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCreditDomain(row))
	}
	return out, nil
}

func (r *Repository) GetCreditNote(ctx context.Context, orgID, id uuid.UUID) (returndomain.CreditNote, error) {
	var row returnmodels.CreditNoteModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		return returndomain.CreditNote{}, err
	}
	return toCreditDomain(row), nil
}

func (r *Repository) ApplyCredit(ctx context.Context, in ApplyCreditInput) (returndomain.CreditNote, error) {
	var out returndomain.CreditNote
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		salePartyIDExpr, err := r.salesPartyIDSelectExpr(ctx, "")
		if err != nil {
			return err
		}
		var note returnmodels.CreditNoteModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("org_id = ? AND id = ?", in.OrgID, in.CreditNoteID).Take(&note).Error; err != nil {
			return err
		}
		if note.Status != "active" {
			return apperror.NewBusinessRule("credit note is not active")
		}
		var sale struct {
			Total      float64
			AmountPaid float64    `gorm:"column:amount_paid"`
			PartyID    *uuid.UUID `gorm:"column:party_id"`
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("sales").Select(fmt.Sprintf("total, amount_paid, %s", salePartyIDExpr)).Where("org_id = ? AND id = ?", in.OrgID, in.SaleID).Take(&sale).Error; err != nil {
			return err
		}
		if sale.PartyID == nil || *sale.PartyID != note.PartyID {
			return apperror.NewBusinessRule("credit note does not belong to sale party")
		}
		pending := sale.Total - sale.AmountPaid
		amount := in.Amount
		if amount <= 0 || amount > note.Balance || amount > pending {
			amount = minFloat(note.Balance, pending)
		}
		if amount <= 0 {
			return apperror.NewBusinessRule("no pending balance to apply")
		}
		if err := tx.Exec(`INSERT INTO payments (id, org_id, reference_type, reference_id, method, amount, notes, received_at, created_by, created_at) VALUES (?, ?, 'sale', ?, 'credit_note', ?, ?, now(), ?, now())`, uuid.New(), in.OrgID, in.SaleID, amount, "credit note applied", in.Actor).Error; err != nil {
			return err
		}
		newAmountPaid := sale.AmountPaid + amount
		if err := tx.Exec(`UPDATE sales SET amount_paid = ?, payment_status = ? WHERE org_id = ? AND id = ?`, newAmountPaid, paymentStatus(newAmountPaid, sale.Total), in.OrgID, in.SaleID).Error; err != nil {
			return err
		}
		usedAmount := note.UsedAmount + amount
		balance := note.Balance - amount
		status := "active"
		if balance <= 0 {
			balance = 0
			status = "used"
		}
		if err := tx.Model(&returnmodels.CreditNoteModel{}).Where("id = ?", note.ID).Updates(map[string]any{"used_amount": usedAmount, "balance": balance, "status": status}).Error; err != nil {
			return err
		}
		out = toCreditDomain(returnmodels.CreditNoteModel{ID: note.ID, OrgID: note.OrgID, Number: note.Number, PartyID: note.PartyID, ReturnID: note.ReturnID, Amount: note.Amount, UsedAmount: usedAmount, Balance: balance, ExpiresAt: note.ExpiresAt, Status: status, CreatedAt: note.CreatedAt})
		return nil
	})
	if err != nil {
		return returndomain.CreditNote{}, err
	}
	return out, nil
}

func toReturnDomain(row returnmodels.ReturnModel, items []returnmodels.ReturnItemModel, partyID *uuid.UUID, partyName string) returndomain.Return {
	out := returndomain.Return{ID: row.ID, OrgID: row.OrgID, Number: row.Number, SaleID: row.SaleID, PartyID: partyID, PartyName: partyName, Reason: row.Reason, Subtotal: row.Subtotal, TaxTotal: row.TaxTotal, Total: row.Total, RefundMethod: row.RefundMethod, Status: row.Status, Notes: row.Notes, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt}
	for _, item := range items {
		out.Items = append(out.Items, returndomain.ReturnItem{ID: item.ID, ReturnID: item.ReturnID, SaleItemID: item.SaleItemID, ProductID: item.ProductID, Description: item.Description, Quantity: item.Quantity, UnitPrice: item.UnitPrice, TaxRate: item.TaxRate, Subtotal: item.Subtotal})
	}
	return out
}

func toCreditDomain(row returnmodels.CreditNoteModel) returndomain.CreditNote {
	return returndomain.CreditNote{ID: row.ID, OrgID: row.OrgID, Number: row.Number, PartyID: row.PartyID, ReturnID: row.ReturnID, Amount: row.Amount, UsedAmount: row.UsedAmount, Balance: row.Balance, ExpiresAt: row.ExpiresAt, Status: row.Status, CreatedAt: row.CreatedAt}
}

func paymentStatus(amountPaid, total float64) string {
	if amountPaid <= 0 {
		return "pending"
	}
	if total <= 0 || amountPaid >= total {
		return "paid"
	}
	return "partial"
}

func defaultString(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func (r *Repository) salesPartyIDSelectExpr(_ context.Context, qualifier string) (string, error) {
	return qualifyColumn(qualifier, "party_id") + " AS party_id", nil
}

func (r *Repository) salesPartyNameSelectExpr(_ context.Context, qualifier string) (string, error) {
	return fmt.Sprintf("COALESCE(%s, '') AS party_name", qualifyColumn(qualifier, "party_name")), nil
}

func qualifyColumn(qualifier, column string) string {
	qualifier = strings.TrimSpace(qualifier)
	if qualifier == "" {
		return column
	}
	return qualifier + "." + column
}
func maxInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
