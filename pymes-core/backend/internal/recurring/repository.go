package recurring

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/recurring/repository/models"
	recurringdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/recurring/usecases/domain"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, orgID uuid.UUID, activeOnly bool, limit int) ([]recurringdomain.RecurringExpense, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&models.RecurringExpenseModel{}).Where("org_id = ?", orgID)
	if activeOnly {
		q = q.Where("is_active = true")
	}
	var rows []models.RecurringExpenseModel
	if err := q.Order("next_due_date ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]recurringdomain.RecurringExpense, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) Create(ctx context.Context, in recurringdomain.RecurringExpense) (recurringdomain.RecurringExpense, error) {
	row := models.RecurringExpenseModel{ID: in.ID, OrgID: in.OrgID, Description: in.Description, Amount: in.Amount, Currency: in.Currency, Category: in.Category, PaymentMethod: in.PaymentMethod, Frequency: in.Frequency, DayOfMonth: in.DayOfMonth, SupplierID: in.SupplierID, IsActive: true, NextDueDate: in.NextDueDate, LastPaidDate: in.LastPaidDate, Notes: in.Notes, CreatedBy: in.CreatedBy, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return recurringdomain.RecurringExpense{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (recurringdomain.RecurringExpense, error) {
	var row models.RecurringExpenseModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		return recurringdomain.RecurringExpense{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in recurringdomain.RecurringExpense) (recurringdomain.RecurringExpense, error) {
	row := models.RecurringExpenseModel{ID: in.ID, OrgID: in.OrgID, Description: in.Description, Amount: in.Amount, Currency: in.Currency, Category: in.Category, PaymentMethod: in.PaymentMethod, Frequency: in.Frequency, DayOfMonth: in.DayOfMonth, SupplierID: in.SupplierID, IsActive: in.IsActive, NextDueDate: in.NextDueDate, LastPaidDate: in.LastPaidDate, Notes: in.Notes, CreatedBy: in.CreatedBy, CreatedAt: in.CreatedAt, UpdatedAt: time.Now().UTC()}
	if err := r.db.WithContext(ctx).Save(&row).Error; err != nil {
		return recurringdomain.RecurringExpense{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Deactivate(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&models.RecurringExpenseModel{}).Where("org_id = ? AND id = ?", orgID, id).Updates(map[string]any{"is_active": false, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) GetCurrency(ctx context.Context, orgID uuid.UUID) string {
	var cur string
	if err := r.db.WithContext(ctx).Table("tenant_settings").Select("currency").Where("org_id = ?", orgID).Take(&cur).Error; err != nil || cur == "" {
		return "ARS"
	}
	return cur
}

func toDomain(row models.RecurringExpenseModel) recurringdomain.RecurringExpense {
	return recurringdomain.RecurringExpense{ID: row.ID, OrgID: row.OrgID, Description: row.Description, Amount: row.Amount, Currency: row.Currency, Category: row.Category, PaymentMethod: row.PaymentMethod, Frequency: row.Frequency, DayOfMonth: row.DayOfMonth, SupplierID: row.SupplierID, IsActive: row.IsActive, NextDueDate: row.NextDueDate, LastPaidDate: row.LastPaidDate, Notes: row.Notes, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}
