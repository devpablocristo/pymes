package payments

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/devpablocristo/core/errors/go/domainerr"
	paymentsdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/payments/usecases/domain"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ListSalePayments(ctx context.Context, orgID, saleID uuid.UUID) ([]paymentsdomain.Payment, error) {
	var rows []paymentsdomain.Payment
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, org_id, reference_type, reference_id, method, amount, notes, received_at, created_by, created_at
		FROM payments
		WHERE org_id = ? AND reference_type = 'sale' AND reference_id = ?
		ORDER BY created_at DESC
	`, orgID, saleID).Scan(&rows).Error
	return rows, err
}

func (r *Repository) CreateSalePayment(ctx context.Context, orgID, saleID uuid.UUID, in paymentsdomain.Payment) (paymentsdomain.Payment, error) {
	var out paymentsdomain.Payment
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sale struct {
			BranchID      *uuid.UUID `gorm:"column:branch_id"`
			Total         float64
			AmountPaid    float64 `gorm:"column:amount_paid"`
			Currency      string
			PaymentMethod string `gorm:"column:payment_method"`
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("sales").Select("branch_id, total, amount_paid, currency, payment_method").Where("org_id = ? AND id = ?", orgID, saleID).Take(&sale).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domainerr.NotFoundf("sale", saleID.String())
			}
			return err
		}
		pending := sale.Total - sale.AmountPaid
		if pending <= 0 {
			return domainerr.Conflict("sale is already fully paid")
		}
		if in.Amount > pending {
			return domainerr.BusinessRule(fmt.Sprintf("payment exceeds pending balance %.2f", pending))
		}
		paymentID := uuid.New()
		createdAt := time.Now().UTC()
		if err := tx.Exec(`
			INSERT INTO payments (id, org_id, reference_type, reference_id, method, amount, notes, received_at, created_by, created_at)
			VALUES (?, ?, 'sale', ?, ?, ?, ?, ?, ?, ?)
		`, paymentID, orgID, saleID, in.Method, in.Amount, in.Notes, in.ReceivedAt.UTC(), in.CreatedBy, createdAt).Error; err != nil {
			return err
		}
		newAmountPaid := sale.AmountPaid + in.Amount
		paymentStatus := "partial"
		if newAmountPaid >= sale.Total {
			paymentStatus = "paid"
		}
		paymentMethod := sale.PaymentMethod
		if paymentMethod == "" {
			paymentMethod = in.Method
		}
		if paymentMethod != in.Method {
			paymentMethod = "mixed"
		}
		if err := tx.Exec(`
			UPDATE sales SET amount_paid = ?, payment_status = ?, payment_method = ? WHERE org_id = ? AND id = ?
		`, newAmountPaid, paymentStatus, paymentMethod, orgID, saleID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			INSERT INTO cash_movements (id, org_id, branch_id, type, amount, currency, category, description, payment_method, reference_type, reference_id, created_by, created_at)
			VALUES (gen_random_uuid(), ?, ?, 'income', ?, ?, 'sale', ?, ?, 'sale', ?, ?, now())
		`, orgID, sale.BranchID, in.Amount, sale.Currency, defaultString(in.Notes, "sale payment"), in.Method, saleID, in.CreatedBy).Error; err != nil {
			return err
		}
		out = paymentsdomain.Payment{ID: paymentID, OrgID: orgID, ReferenceType: "sale", ReferenceID: saleID, Method: in.Method, Amount: in.Amount, Notes: in.Notes, ReceivedAt: in.ReceivedAt.UTC(), CreatedBy: in.CreatedBy, CreatedAt: createdAt}
		return nil
	})
	if err != nil {
		return paymentsdomain.Payment{}, err
	}
	return out, nil
}

func defaultString(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}
