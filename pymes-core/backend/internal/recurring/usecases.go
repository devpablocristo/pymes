package recurring

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/errors/go/domainerr"
	archive "github.com/devpablocristo/modules/crud/archive/go/archive"
	recurringdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/recurring/usecases/domain"
)

type RepositoryPort interface {
	List(ctx context.Context, orgID uuid.UUID, activeOnly bool, limit int) ([]recurringdomain.RecurringExpense, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]recurringdomain.RecurringExpense, error)
	Create(ctx context.Context, in recurringdomain.RecurringExpense) (recurringdomain.RecurringExpense, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (recurringdomain.RecurringExpense, error)
	Update(ctx context.Context, in recurringdomain.RecurringExpense) (recurringdomain.RecurringExpense, error)
	Deactivate(ctx context.Context, orgID, id uuid.UUID) error
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
	Restore(ctx context.Context, orgID, id uuid.UUID) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID) error
	GetCurrency(ctx context.Context, orgID uuid.UUID) string
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

func (u *Usecases) List(ctx context.Context, orgID uuid.UUID, activeOnly bool, limit int) ([]recurringdomain.RecurringExpense, error) {
	return u.repo.List(ctx, orgID, activeOnly, limit)
}

func (u *Usecases) ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]recurringdomain.RecurringExpense, error) {
	return u.repo.ListArchived(ctx, orgID, limit)
}

func (u *Usecases) Create(ctx context.Context, in recurringdomain.RecurringExpense) (recurringdomain.RecurringExpense, error) {
	prepared, err := prepareRecurring(in, true, u.repo.GetCurrency(ctx, in.OrgID))
	if err != nil {
		return recurringdomain.RecurringExpense{}, err
	}
	if prepared.ID == uuid.Nil {
		prepared.ID = uuid.New()
	}
	out, err := u.repo.Create(ctx, prepared)
	if err != nil {
		return recurringdomain.RecurringExpense{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), in.CreatedBy, "recurring_expense.created", "recurring_expense", out.ID.String(), map[string]any{"amount": out.Amount})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (recurringdomain.RecurringExpense, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return recurringdomain.RecurringExpense{}, domainerr.NotFoundf("recurring_expense", id.String())
		}
		return recurringdomain.RecurringExpense{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, in recurringdomain.RecurringExpense, actor string) (recurringdomain.RecurringExpense, error) {
	current, err := u.repo.GetByID(ctx, in.OrgID, in.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return recurringdomain.RecurringExpense{}, domainerr.NotFoundf("recurring_expense", in.ID.String())
		}
		return recurringdomain.RecurringExpense{}, err
	}
	if err := archive.IfArchived(current.ArchivedAt, "recurring_expense"); err != nil {
		return recurringdomain.RecurringExpense{}, err
	}
	mergeRecurring(&current, in)
	current, err = prepareRecurring(current, false, u.repo.GetCurrency(ctx, current.OrgID))
	if err != nil {
		return recurringdomain.RecurringExpense{}, err
	}
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		return recurringdomain.RecurringExpense{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), actor, "recurring_expense.updated", "recurring_expense", out.ID.String(), map[string]any{"amount": out.Amount})
	}
	return out, nil
}

func (u *Usecases) Deactivate(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Deactivate(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("recurring_expense", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "recurring_expense.deactivated", "recurring_expense", id.String(), nil)
	}
	return nil
}

func (u *Usecases) SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.SoftDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("recurring_expense", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "recurring_expense.archived", "recurring_expense", id.String(), nil)
	}
	return nil
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("recurring_expense", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "recurring_expense.restored", "recurring_expense", id.String(), nil)
	}
	return nil
}

func (u *Usecases) HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.HardDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("recurring_expense", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "recurring_expense.hard_deleted", "recurring_expense", id.String(), nil)
	}
	return nil
}

var allowedFrequencies = map[string]struct{}{
	"weekly": {}, "biweekly": {}, "monthly": {}, "quarterly": {}, "yearly": {},
}

func prepareRecurring(in recurringdomain.RecurringExpense, creating bool, defaultCurrency string) (recurringdomain.RecurringExpense, error) {
	if creating && in.OrgID == uuid.Nil {
		return recurringdomain.RecurringExpense{}, domainerr.Validation("org_id is required")
	}
	if strings.TrimSpace(in.Description) == "" {
		return recurringdomain.RecurringExpense{}, domainerr.Validation("description is required")
	}
	if in.Amount <= 0 {
		return recurringdomain.RecurringExpense{}, domainerr.Validation("amount must be > 0")
	}
	in.Currency = defaultString(in.Currency, defaultCurrency)
	in.Category = defaultString(in.Category, "other")
	in.PaymentMethod = defaultString(in.PaymentMethod, "transfer")
	in.Frequency = defaultString(strings.ToLower(in.Frequency), "monthly")
	if _, ok := allowedFrequencies[in.Frequency]; !ok {
		return recurringdomain.RecurringExpense{}, domainerr.Validation("invalid frequency")
	}
	if in.DayOfMonth <= 0 {
		in.DayOfMonth = 1
	}
	if in.DayOfMonth > 28 {
		in.DayOfMonth = 28
	}
	if in.NextDueDate.IsZero() {
		now := time.Now().UTC()
		in.NextDueDate = time.Date(now.Year(), now.Month(), in.DayOfMonth, 0, 0, 0, 0, time.UTC)
		if in.NextDueDate.Before(now.Truncate(24 * time.Hour)) {
			in.NextDueDate = nextRecurringDate(in.NextDueDate, in.Frequency, in.DayOfMonth)
		}
	}
	return in, nil
}

func mergeRecurring(dst *recurringdomain.RecurringExpense, patch recurringdomain.RecurringExpense) {
	if strings.TrimSpace(patch.Description) != "" {
		dst.Description = strings.TrimSpace(patch.Description)
	}
	if patch.Amount > 0 {
		dst.Amount = patch.Amount
	}
	if patch.Currency != "" {
		dst.Currency = strings.TrimSpace(patch.Currency)
	}
	if patch.Category != "" {
		dst.Category = strings.TrimSpace(patch.Category)
	}
	if patch.PaymentMethod != "" {
		dst.PaymentMethod = strings.TrimSpace(patch.PaymentMethod)
	}
	if patch.Frequency != "" {
		dst.Frequency = strings.TrimSpace(patch.Frequency)
	}
	if patch.DayOfMonth > 0 {
		dst.DayOfMonth = patch.DayOfMonth
	}
	if patch.SupplierID != nil {
		dst.SupplierID = patch.SupplierID
	}
	if !patch.NextDueDate.IsZero() {
		dst.NextDueDate = patch.NextDueDate
	}
	if patch.Notes != "" {
		dst.Notes = strings.TrimSpace(patch.Notes)
	}
	dst.IsActive = patch.IsActive || dst.IsActive
	dst.IsFavorite = patch.IsFavorite || dst.IsFavorite
	if patch.Tags != nil {
		dst.Tags = patch.Tags
	}
}

func nextRecurringDate(base time.Time, frequency string, dayOfMonth int) time.Time {
	switch frequency {
	case "weekly":
		return base.AddDate(0, 0, 7)
	case "biweekly":
		return base.AddDate(0, 0, 14)
	case "quarterly":
		return time.Date(base.Year(), base.Month()+3, dayOfMonth, 0, 0, 0, 0, time.UTC)
	case "yearly":
		return time.Date(base.Year()+1, base.Month(), dayOfMonth, 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(base.Year(), base.Month()+1, dayOfMonth, 0, 0, 0, 0, time.UTC)
	}
}

func defaultString(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}
