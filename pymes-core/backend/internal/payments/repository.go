package payments

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/devpablocristo/core/http/go/pagination"
	utils "github.com/devpablocristo/core/validate/go/stringutil"
	paymentsdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/payments/usecases/domain"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type paymentRow struct {
	ID            uuid.UUID      `gorm:"column:id"`
	TenantID      uuid.UUID      `gorm:"column:tenant_id"`
	ReferenceType string         `gorm:"column:reference_type"`
	ReferenceID   uuid.UUID      `gorm:"column:reference_id"`
	Method        string         `gorm:"column:method"`
	Amount        float64        `gorm:"column:amount"`
	Notes         string         `gorm:"column:notes"`
	ReceivedAt    time.Time      `gorm:"column:received_at"`
	IsFavorite    bool           `gorm:"column:is_favorite"`
	Tags          pq.StringArray `gorm:"column:tags;type:text[]"`
	DeletedAt     *time.Time     `gorm:"column:deleted_at"`
	CreatedBy     string         `gorm:"column:created_by"`
	CreatedAt     time.Time      `gorm:"column:created_at"`
}

func (p paymentRow) toDomain() paymentsdomain.Payment {
	return paymentsdomain.Payment{
		ID:            p.ID,
		TenantID:      p.TenantID,
		ReferenceType: p.ReferenceType,
		ReferenceID:   p.ReferenceID,
		Method:        p.Method,
		Amount:        p.Amount,
		Notes:         p.Notes,
		ReceivedAt:    p.ReceivedAt,
		IsFavorite:    p.IsFavorite,
		Tags:          append([]string(nil), p.Tags...),
		ArchivedAt:    p.DeletedAt,
		CreatedBy:     p.CreatedBy,
		CreatedAt:     p.CreatedAt,
	}
}

func (r *Repository) ListSalePayments(ctx context.Context, tenantID, saleID uuid.UUID) ([]paymentsdomain.Payment, error) {
	var rows []paymentRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT id, tenant_id, reference_type, reference_id, method, amount, notes, received_at, is_favorite, tags, deleted_at, created_by, created_at
		FROM payments
		WHERE tenant_id = ? AND reference_type = 'sale' AND reference_id = ? AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, tenantID, saleID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]paymentsdomain.Payment, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toDomain())
	}
	return out, nil
}

func (r *Repository) ListArchived(ctx context.Context, tenantID uuid.UUID, limit int) ([]paymentsdomain.Payment, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	var rows []paymentRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT id, tenant_id, reference_type, reference_id, method, amount, notes, received_at, is_favorite, tags, deleted_at, created_by, created_at
		FROM payments
		WHERE tenant_id = ? AND deleted_at IS NOT NULL
		ORDER BY deleted_at DESC
		LIMIT ?
	`, tenantID, limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]paymentsdomain.Payment, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toDomain())
	}
	return out, nil
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (paymentsdomain.Payment, error) {
	var row paymentRow
	res := r.db.WithContext(ctx).Raw(`
		SELECT id, tenant_id, reference_type, reference_id, method, amount, notes, received_at, is_favorite, tags, deleted_at, created_by, created_at
		FROM payments
		WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL
	`, tenantID, id).Scan(&row)
	if res.Error != nil {
		return paymentsdomain.Payment{}, res.Error
	}
	if res.RowsAffected == 0 {
		return paymentsdomain.Payment{}, gorm.ErrRecordNotFound
	}
	return row.toDomain(), nil
}

func (r *Repository) Update(ctx context.Context, in paymentsdomain.Payment) (paymentsdomain.Payment, error) {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE payments
		SET notes = ?, is_favorite = ?, tags = ?
		WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL
	`, in.Notes, in.IsFavorite, pq.StringArray(utils.NormalizeTags(in.Tags)), in.TenantID, in.ID)
	if res.Error != nil {
		return paymentsdomain.Payment{}, res.Error
	}
	if res.RowsAffected == 0 {
		return paymentsdomain.Payment{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.TenantID, in.ID)
}

func (r *Repository) SoftDelete(ctx context.Context, tenantID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE payments SET deleted_at = ? WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL
	`, time.Now().UTC(), tenantID, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) Restore(ctx context.Context, tenantID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE payments SET deleted_at = NULL WHERE tenant_id = ? AND id = ? AND deleted_at IS NOT NULL
	`, tenantID, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) HardDelete(ctx context.Context, tenantID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Exec(`DELETE FROM payments WHERE tenant_id = ? AND id = ?`, tenantID, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) CreateSalePayment(ctx context.Context, tenantID, saleID uuid.UUID, in paymentsdomain.Payment) (paymentsdomain.Payment, error) {
	var out paymentsdomain.Payment
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sale struct {
			BranchID      *uuid.UUID `gorm:"column:branch_id"`
			Total         float64
			AmountPaid    float64 `gorm:"column:amount_paid"`
			Currency      string
			PaymentMethod string `gorm:"column:payment_method"`
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("sales").Select("branch_id, total, amount_paid, currency, payment_method").Where("tenant_id = ? AND id = ?", tenantID, saleID).Take(&sale).Error; err != nil {
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
			INSERT INTO payments (id, tenant_id, reference_type, reference_id, method, amount, notes, received_at, is_favorite, tags, created_by, created_at)
			VALUES (?, ?, 'sale', ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, paymentID, tenantID, saleID, in.Method, in.Amount, in.Notes, in.ReceivedAt.UTC(), in.IsFavorite, pq.StringArray(utils.NormalizeTags(in.Tags)), in.CreatedBy, createdAt).Error; err != nil {
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
			UPDATE sales SET amount_paid = ?, payment_status = ?, payment_method = ? WHERE tenant_id = ? AND id = ?
		`, newAmountPaid, paymentStatus, paymentMethod, tenantID, saleID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			INSERT INTO cash_movements (id, tenant_id, branch_id, type, amount, currency, category, description, payment_method, reference_type, reference_id, created_by, created_at)
			VALUES (gen_random_uuid(), ?, ?, 'income', ?, ?, 'sale', ?, ?, 'sale', ?, ?, now())
		`, tenantID, sale.BranchID, in.Amount, sale.Currency, defaultString(in.Notes, "sale payment"), in.Method, saleID, in.CreatedBy).Error; err != nil {
			return err
		}
		out = paymentsdomain.Payment{ID: paymentID, TenantID: tenantID, ReferenceType: "sale", ReferenceID: saleID, Method: in.Method, Amount: in.Amount, Notes: in.Notes, ReceivedAt: in.ReceivedAt.UTC(), IsFavorite: in.IsFavorite, Tags: append([]string(nil), utils.NormalizeTags(in.Tags)...), CreatedBy: in.CreatedBy, CreatedAt: createdAt}
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
