// Package publicapi implements public website and booking data access.
package publicapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrOrgNotFound     = errors.New("org not found")
	ErrInvalidInput    = errors.New("invalid input")
	ErrSlotUnavailable = errors.New("slot unavailable")
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type BusinessInfo struct {
	OrgID               uuid.UUID `json:"org_id"`
	Name                string    `json:"name"`
	Slug                string    `json:"slug"`
	BusinessName        string    `json:"business_name"`
	BusinessAddress     string    `json:"business_address"`
	BusinessPhone       string    `json:"business_phone"`
	BusinessEmail       string    `json:"business_email"`
	AppointmentsEnabled bool      `json:"appointments_enabled"`
}

type PublicService struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Unit        string    `json:"unit"`
	Price       float64   `json:"price"`
	Currency    string    `json:"currency"`
}

type AvailabilitySlot struct {
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
	Remaining int       `json:"remaining"`
}

type BookInput struct {
	CustomerName  string
	CustomerPhone string
	Title         string
	StartAt       time.Time
	Duration      int
}

type AppointmentPublic struct {
	ID            uuid.UUID `json:"id"`
	CustomerName  string    `json:"party_name" gorm:"column:party_name"`
	CustomerPhone string    `json:"party_phone" gorm:"column:party_phone"`
	Title         string    `json:"title"`
	Status        string    `json:"status"`
	StartAt       time.Time `json:"start_at"`
	EndAt         time.Time `json:"end_at"`
	Duration      int       `json:"duration"`
}

func (r *Repository) ResolveOrgID(ctx context.Context, ref string) (uuid.UUID, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return uuid.Nil, ErrOrgNotFound
	}

	if parsed, err := uuid.Parse(trimmed); err == nil {
		var row struct {
			ID uuid.UUID `gorm:"column:id"`
		}
		err = r.db.WithContext(ctx).
			Table("orgs").
			Select("id").
			Where("id = ?", parsed).
			Take(&row).Error
		if err == nil {
			return row.ID, nil
		}
	}

	var row struct {
		ID uuid.UUID
	}
	err := r.db.WithContext(ctx).
		Table("orgs").
		Select("id").
		Where("slug = ?", trimmed).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uuid.Nil, ErrOrgNotFound
		}
		return uuid.Nil, err
	}
	return row.ID, nil
}

func (r *Repository) GetBusinessInfo(ctx context.Context, orgID uuid.UUID) (BusinessInfo, error) {
	var row struct {
		OrgID               uuid.UUID
		Name                string
		Slug                string
		BusinessName        string
		BusinessAddress     string
		BusinessPhone       string
		BusinessEmail       string
		AppointmentsEnabled bool
	}

	err := r.db.WithContext(ctx).
		Table("orgs o").
		Select(`
			o.id as org_id,
			o.name,
			o.slug,
			COALESCE(ts.business_name, '') as business_name,
			COALESCE(ts.business_address, '') as business_address,
			COALESCE(ts.business_phone, '') as business_phone,
			COALESCE(ts.business_email, '') as business_email,
			COALESCE(ts.appointments_enabled, false) as appointments_enabled
		`).
		Joins("LEFT JOIN tenant_settings ts ON ts.org_id = o.id").
		Where("o.id = ?", orgID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return BusinessInfo{}, ErrOrgNotFound
		}
		return BusinessInfo{}, err
	}

	businessName := strings.TrimSpace(row.BusinessName)
	if businessName == "" {
		businessName = row.Name
	}

	return BusinessInfo{
		OrgID:               row.OrgID,
		Name:                row.Name,
		Slug:                row.Slug,
		BusinessName:        businessName,
		BusinessAddress:     row.BusinessAddress,
		BusinessPhone:       row.BusinessPhone,
		BusinessEmail:       row.BusinessEmail,
		AppointmentsEnabled: row.AppointmentsEnabled,
	}, nil
}

func (r *Repository) ListPublicServices(ctx context.Context, orgID uuid.UUID, limit int) ([]PublicService, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var rows []PublicService
	err := r.db.WithContext(ctx).
		Table("products").
		Select("id, name, type, description, unit, price, COALESCE(price_currency, 'ARS') as currency").
		Where("org_id = ? AND deleted_at IS NULL", orgID).
		Order("name ASC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) GetAvailability(ctx context.Context, orgID uuid.UUID, day time.Time, duration int) ([]AvailabilitySlot, error) {
	if duration <= 0 {
		duration = 60
	}
	if duration > 720 {
		return nil, ErrInvalidInput
	}

	slots, err := r.listSlotConfigs(ctx, orgID, int(day.Weekday()))
	if err != nil {
		return nil, err
	}
	if len(slots) == 0 {
		slots = []slotConfig{{StartHHMM: "09:00", EndHHMM: "18:00", SlotMinutes: 60, MaxPerSlot: 1}}
	}

	out := make([]AvailabilitySlot, 0)
	for _, slot := range slots {
		start, err := composeDayTime(day, slot.StartHHMM)
		if err != nil {
			continue
		}
		end, err := composeDayTime(day, slot.EndHHMM)
		if err != nil {
			continue
		}
		if !end.After(start) {
			continue
		}

		step := slot.SlotMinutes
		if step <= 0 {
			step = duration
		}
		for cursor := start; cursor.Add(time.Duration(duration)*time.Minute).Equal(end) || cursor.Add(time.Duration(duration)*time.Minute).Before(end); cursor = cursor.Add(time.Duration(step) * time.Minute) {
			candidateEnd := cursor.Add(time.Duration(duration) * time.Minute)
			count, err := r.countOverlaps(ctx, orgID, cursor, candidateEnd)
			if err != nil {
				return nil, err
			}
			remaining := slot.MaxPerSlot - int(count)
			if remaining > 0 {
				out = append(out, AvailabilitySlot{StartAt: cursor, EndAt: candidateEnd, Remaining: remaining})
			}
		}
	}

	return out, nil
}

func (r *Repository) Book(ctx context.Context, orgID uuid.UUID, in BookInput) (AppointmentPublic, error) {
	name := strings.TrimSpace(in.CustomerName)
	phone := strings.TrimSpace(in.CustomerPhone)
	title := strings.TrimSpace(in.Title)
	if name == "" || phone == "" || title == "" {
		return AppointmentPublic{}, ErrInvalidInput
	}
	if in.Duration <= 0 {
		in.Duration = 60
	}
	if in.Duration > 720 {
		return AppointmentPublic{}, ErrInvalidInput
	}

	maxPerSlot := 1
	if v, err := r.findMaxPerSlot(ctx, orgID, in.StartAt); err != nil {
		return AppointmentPublic{}, err
	} else if v > 0 {
		maxPerSlot = v
	}

	endAt := in.StartAt.Add(time.Duration(in.Duration) * time.Minute)
	overlaps, err := r.countOverlaps(ctx, orgID, in.StartAt, endAt)
	if err != nil {
		return AppointmentPublic{}, err
	}
	if int(overlaps) >= maxPerSlot {
		return AppointmentPublic{}, ErrSlotUnavailable
	}

	appointment := AppointmentPublic{
		ID:            uuid.New(),
		CustomerName:  name,
		CustomerPhone: phone,
		Title:         title,
		Status:        "scheduled",
		StartAt:       in.StartAt.UTC(),
		EndAt:         endAt.UTC(),
		Duration:      in.Duration,
	}

	err = r.db.WithContext(ctx).Table("appointments").Create(map[string]any{
		"id":          appointment.ID,
		"org_id":      orgID,
		"party_name":  appointment.CustomerName,
		"party_phone": appointment.CustomerPhone,
		"title":       appointment.Title,
		"status":      appointment.Status,
		"start_at":    appointment.StartAt,
		"end_at":      appointment.EndAt,
		"duration":    appointment.Duration,
		"created_by":  "public-api",
		"created_at":  time.Now().UTC(),
		"updated_at":  time.Now().UTC(),
	}).Error
	if err != nil {
		return AppointmentPublic{}, err
	}
	return appointment, nil
}

func (r *Repository) ListByPhone(ctx context.Context, orgID uuid.UUID, phone string, limit int) ([]AppointmentPublic, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	phoneDigits := digitsOnly(phone)
	if phoneDigits == "" {
		return nil, ErrInvalidInput
	}

	var rows []AppointmentPublic
	err := r.db.WithContext(ctx).
		Table("appointments").
		Select("id, party_name, party_phone, title, status, start_at, end_at, duration").
		Where("org_id = ? AND regexp_replace(party_phone, '[^0-9]', '', 'g') = ?", orgID, phoneDigits).
		Order("start_at DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

type slotConfig struct {
	StartHHMM   string `gorm:"column:start_hhmm"`
	EndHHMM     string `gorm:"column:end_hhmm"`
	SlotMinutes int    `gorm:"column:slot_minutes"`
	MaxPerSlot  int    `gorm:"column:max_per_slot"`
}

func (r *Repository) listSlotConfigs(ctx context.Context, orgID uuid.UUID, dayOfWeek int) ([]slotConfig, error) {
	var rows []slotConfig
	err := r.db.WithContext(ctx).
		Table("appointment_slots").
		Select("to_char(start_time, 'HH24:MI') as start_hhmm, to_char(end_time, 'HH24:MI') as end_hhmm, slot_minutes, max_per_slot").
		Where("org_id = ? AND day_of_week = ?", orgID, dayOfWeek).
		Order("start_time ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) countOverlaps(ctx context.Context, orgID uuid.UUID, startAt, endAt time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("appointments").
		Where("org_id = ?", orgID).
		Where("status IN ?", []string{"scheduled", "confirmed", "in_progress"}).
		Where("start_at < ? AND end_at > ?", endAt.UTC(), startAt.UTC()).
		Count(&count).Error
	return count, err
}

func (r *Repository) findMaxPerSlot(ctx context.Context, orgID uuid.UUID, startAt time.Time) (int, error) {
	timeText := startAt.UTC().Format("15:04:05")
	var result struct {
		MaxPerSlot int
	}
	err := r.db.WithContext(ctx).
		Raw(`
			SELECT max_per_slot
			FROM appointment_slots
			WHERE org_id = ?
			  AND day_of_week = ?
			  AND start_time <= ?::time
			  AND end_time > ?::time
			ORDER BY start_time ASC
			LIMIT 1
		`, orgID, int(startAt.Weekday()), timeText, timeText).
		Scan(&result).Error
	if err != nil {
		return 0, err
	}
	return result.MaxPerSlot, nil
}

func composeDayTime(day time.Time, hhmm string) (time.Time, error) {
	parsed, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", hhmm, err)
	}
	return time.Date(day.Year(), day.Month(), day.Day(), parsed.Hour(), parsed.Minute(), 0, 0, time.UTC), nil
}

func digitsOnly(v string) string {
	var b strings.Builder
	for _, r := range v {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
