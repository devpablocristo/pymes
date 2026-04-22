package cashflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/errors/go/domainerr"
	archive "github.com/devpablocristo/modules/crud/archive/go/archive"
	cashdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/cashflow/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]cashdomain.CashMovement, int64, bool, *uuid.UUID, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]cashdomain.CashMovement, error)
	Create(ctx context.Context, in cashdomain.CashMovement) (cashdomain.CashMovement, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (cashdomain.CashMovement, error)
	Update(ctx context.Context, in cashdomain.CashMovement) (cashdomain.CashMovement, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
	Restore(ctx context.Context, orgID, id uuid.UUID) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID) error
	GetCurrency(ctx context.Context, orgID uuid.UUID) string
	Summary(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) (cashdomain.CashSummary, error)
	DailySummary(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, days int) ([]cashdomain.CashSummary, error)
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type Usecases struct {
	repo  RepositoryPort
	audit AuditPort
}

func NewUsecases(repo RepositoryPort, audit AuditPort) *Usecases {
	return &Usecases{repo: repo, audit: audit}
}

func (u *Usecases) List(ctx context.Context, p ListParams) ([]cashdomain.CashMovement, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]cashdomain.CashMovement, error) {
	return u.repo.ListArchived(ctx, orgID, limit)
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (cashdomain.CashMovement, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return cashdomain.CashMovement{}, domainerr.NotFoundf("cash_movement", id.String())
		}
		return cashdomain.CashMovement{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, in cashdomain.CashMovement, actor string) (cashdomain.CashMovement, error) {
	current, err := u.repo.GetByID(ctx, in.OrgID, in.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return cashdomain.CashMovement{}, domainerr.NotFoundf("cash_movement", in.ID.String())
		}
		return cashdomain.CashMovement{}, err
	}
	if err := archive.IfArchived(current.ArchivedAt, "cash_movement"); err != nil {
		return cashdomain.CashMovement{}, err
	}
	// El tipo e importe son inmutables: solo editamos metadatos (favoritos, tags,
	// categoría, descripción, medio de pago) para mantener la integridad del log.
	current.Category = strings.TrimSpace(in.Category)
	current.Description = strings.TrimSpace(in.Description)
	current.PaymentMethod = strings.TrimSpace(in.PaymentMethod)
	current.IsFavorite = in.IsFavorite
	current.Tags = in.Tags
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return cashdomain.CashMovement{}, domainerr.NotFoundf("cash_movement", in.ID.String())
		}
		return cashdomain.CashMovement{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "cashflow.updated", "cash_movement", out.ID.String(), nil)
	}
	return out, nil
}

func (u *Usecases) SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.SoftDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("cash_movement", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "cashflow.archived", "cash_movement", id.String(), nil)
	}
	return nil
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("cash_movement", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "cashflow.restored", "cash_movement", id.String(), nil)
	}
	return nil
}

func (u *Usecases) HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.HardDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("cash_movement", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "cashflow.hard_deleted", "cash_movement", id.String(), nil)
	}
	return nil
}

func (u *Usecases) CreateManual(ctx context.Context, in cashdomain.CashMovement) (cashdomain.CashMovement, error) {
	in.Type = strings.TrimSpace(in.Type)
	if in.Type != "income" && in.Type != "expense" {
		return cashdomain.CashMovement{}, fmt.Errorf("invalid type: %w", httperrors.ErrBadInput)
	}
	if in.Amount <= 0 {
		return cashdomain.CashMovement{}, fmt.Errorf("amount must be > 0: %w", httperrors.ErrBadInput)
	}
	if strings.TrimSpace(in.ReferenceType) == "" {
		in.ReferenceType = "manual"
	}
	if strings.TrimSpace(in.Currency) == "" {
		in.Currency = u.repo.GetCurrency(ctx, in.OrgID)
	}
	if strings.TrimSpace(in.Category) == "" {
		in.Category = "other"
	}
	if strings.TrimSpace(in.PaymentMethod) == "" {
		in.PaymentMethod = "cash"
	}
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return cashdomain.CashMovement{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), in.CreatedBy, "cashflow.created", "cash_movement", out.ID.String(), map[string]any{
			"type":   out.Type,
			"amount": out.Amount,
		})
	}
	return out, nil
}

func (u *Usecases) Summary(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) (cashdomain.CashSummary, error) {
	if to.Before(from) {
		return cashdomain.CashSummary{}, fmt.Errorf("invalid date range: %w", httperrors.ErrBadInput)
	}
	return u.repo.Summary(ctx, orgID, branchID, from, to)
}

func (u *Usecases) DailySummary(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, days int) ([]cashdomain.CashSummary, error) {
	if days <= 0 {
		days = 30
	}
	return u.repo.DailySummary(ctx, orgID, branchID, days)
}

func (u *Usecases) RecordSaleIncome(ctx context.Context, orgID, saleID uuid.UUID, branchID *uuid.UUID, amount float64, currency, paymentMethod, actor string) error {
	out, err := u.repo.Create(ctx, cashdomain.CashMovement{
		OrgID:         orgID,
		BranchID:      branchID,
		Type:          "income",
		Amount:        amount,
		Currency:      coalesce(currency, u.repo.GetCurrency(ctx, orgID)),
		Category:      "sale",
		Description:   "sale income",
		PaymentMethod: coalesce(paymentMethod, "cash"),
		ReferenceType: "sale",
		ReferenceID:   &saleID,
		CreatedBy:     actor,
	})
	if err == nil && u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "cashflow.sale_income", "cash_movement", out.ID.String(), map[string]any{
			"sale_id": saleID.String(),
			"amount":  amount,
		})
	}
	return err
}

func (u *Usecases) RecordSaleVoidExpense(ctx context.Context, orgID, saleID uuid.UUID, branchID *uuid.UUID, amount float64, currency, paymentMethod, actor string) error {
	out, err := u.repo.Create(ctx, cashdomain.CashMovement{
		OrgID:         orgID,
		BranchID:      branchID,
		Type:          "expense",
		Amount:        amount,
		Currency:      coalesce(currency, u.repo.GetCurrency(ctx, orgID)),
		Category:      "sale",
		Description:   "sale void reversal",
		PaymentMethod: coalesce(paymentMethod, "cash"),
		ReferenceType: "sale",
		ReferenceID:   &saleID,
		CreatedBy:     actor,
	})
	if err == nil && u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "cashflow.sale_void", "cash_movement", out.ID.String(), map[string]any{
			"sale_id": saleID.String(),
			"amount":  amount,
		})
	}
	return err
}

func coalesce(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}
