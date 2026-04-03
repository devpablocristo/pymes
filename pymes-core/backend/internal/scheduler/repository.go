package scheduler

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ListAutoFetchRateOrgs(ctx context.Context) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.db.WithContext(ctx).Table("tenant_settings").Select("org_id").Where("auto_fetch_rates = true").Scan(&ids).Error
	return ids, err
}

func (r *Repository) UpsertExchangeRate(ctx context.Context, orgID uuid.UUID, fromCurrency, toCurrency, rateType string, buyRate, sellRate float64, source string, rateDate time.Time) error {
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO exchange_rates (id, org_id, from_currency, to_currency, rate_type, buy_rate, sell_rate, source, rate_date, created_at)
		VALUES (gen_random_uuid(), ?, ?, ?, ?, ?, ?, ?, ?, now())
		ON CONFLICT (org_id, from_currency, to_currency, rate_type, rate_date)
		DO UPDATE SET buy_rate = EXCLUDED.buy_rate, sell_rate = EXCLUDED.sell_rate, source = EXCLUDED.source, created_at = now()
	`, orgID, fromCurrency, toCurrency, rateType, buyRate, sellRate, source, time.Date(rateDate.Year(), rateDate.Month(), rateDate.Day(), 0, 0, 0, 0, time.UTC)).Error
}

func (r *Repository) ListDueRecurring(ctx context.Context, day time.Time) ([]RecurringDue, error) {
	var rows []RecurringDue
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, org_id, description, amount, currency, category, payment_method, frequency, day_of_month, next_due_date
		FROM recurring_expenses
		WHERE is_active = true AND next_due_date <= ? AND (last_paid_date IS NULL OR last_paid_date < next_due_date)
		ORDER BY next_due_date ASC
	`, day).Scan(&rows).Error
	return rows, err
}

func (r *Repository) ApplyRecurringExpense(ctx context.Context, item RecurringDue, paidAt, nextDue time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			INSERT INTO cash_movements (id, org_id, type, amount, currency, category, description, payment_method, reference_type, reference_id, created_by, created_at)
			VALUES (gen_random_uuid(), ?, 'expense', ?, ?, ?, ?, ?, 'recurring_expense', ?, 'scheduler', now())
		`, item.OrgID, item.Amount, item.Currency, item.Category, item.Description, item.PaymentMethod, item.ID).Error; err != nil {
			return err
		}
		return tx.Exec(`
			UPDATE recurring_expenses
			SET last_paid_date = ?, next_due_date = ?, updated_at = now()
			WHERE id = ?
		`, paidAt, nextDue, item.ID).Error
	})
}

func (r *Repository) ListDueSchedulingReminders(ctx context.Context, now time.Time, limit int) ([]SchedulingReminderDue, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	var rows []SchedulingReminderDue
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			b.org_id,
			COALESCE(o.slug, b.org_id::text) AS org_slug,
			b.id AS booking_id,
			b.customer_name,
			b.customer_email,
			COALESCE(s.name, b.reference, 'Booking') AS service_name,
			COALESCE(br.name, '') AS branch_name,
			b.status,
			b.start_at
		FROM scheduling_bookings b
		JOIN orgs o ON o.id = b.org_id
		JOIN tenant_settings ts ON ts.org_id = b.org_id
		LEFT JOIN scheduling_services s ON s.id = b.service_id
		LEFT JOIN scheduling_branches br ON br.id = b.branch_id
		WHERE ts.scheduling_enabled = true
		  AND b.customer_email <> ''
		  AND b.reminder_sent_at IS NULL
		  AND b.status IN ('pending_confirmation', 'confirmed')
		  AND b.start_at >= ?
		  AND b.start_at <= (? + make_interval(hours => GREATEST(ts.appointment_reminder_hours, 0)))
		ORDER BY b.start_at ASC
		LIMIT ?
	`, now.UTC(), now.UTC(), limit).Scan(&rows).Error
	return rows, err
}

func (r *Repository) RecordRun(ctx context.Context, task, status, errorMessage string, nextRunAt time.Time) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "task_name"}},
		DoUpdates: clause.Assignments(map[string]any{
			"last_run_at":   time.Now().UTC(),
			"next_run_at":   nextRunAt.UTC(),
			"status":        status,
			"error_message": errorMessage,
		}),
	}).Table("scheduler_runs").Create(map[string]any{
		"task_name":     task,
		"last_run_at":   time.Now().UTC(),
		"next_run_at":   nextRunAt.UTC(),
		"status":        status,
		"error_message": errorMessage,
	}).Error
}
