// Package publicapi implements public website and booking data access.
package publicapi

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"gorm.io/gorm"

	schedulingdomain "github.com/devpablocristo/modules/scheduling/go/domain"
)

var (
	ErrOrgNotFound     = errors.New("org not found")
	ErrInvalidInput    = errors.New("invalid input")
	ErrSlotUnavailable = errors.New("slot unavailable")
)

type schedulingPort interface {
	ListBranches(ctx context.Context, orgID uuid.UUID) ([]schedulingdomain.Branch, error)
	ListServices(ctx context.Context, orgID uuid.UUID) ([]schedulingdomain.Service, error)
	ListResources(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]schedulingdomain.Resource, error)
	ListAvailableSlots(ctx context.Context, orgID uuid.UUID, query schedulingdomain.SlotQuery) ([]schedulingdomain.TimeSlot, error)
	CreateBooking(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.CreateBookingInput) (schedulingdomain.Booking, error)
}

type Repository struct {
	db         *gorm.DB
	scheduling schedulingPort
}

func NewRepository(db *gorm.DB, scheduling schedulingPort) *Repository {
	return &Repository{db: db, scheduling: scheduling}
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

type AvailabilityQuery struct {
	Date       time.Time
	Duration   int
	BranchID   *uuid.UUID
	ServiceID  *uuid.UUID
	ResourceID *uuid.UUID
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

type schedulingSelection struct {
	Branch   schedulingdomain.Branch
	Service  schedulingdomain.Service
	Resource *schedulingdomain.Resource
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
	if items, ok, err := r.listSchedulingPublicServices(ctx, orgID, limit); err != nil {
		return nil, err
	} else if ok {
		return items, nil
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

func (r *Repository) GetAvailability(ctx context.Context, orgID uuid.UUID, query AvailabilityQuery) ([]AvailabilitySlot, error) {
	if query.Duration < 0 || query.Duration > 720 {
		return nil, ErrInvalidInput
	}
	if selection, ok, err := r.resolveSchedulingSelection(ctx, orgID, query.BranchID, query.ServiceID, query.ResourceID); err != nil {
		return nil, err
	} else if ok {
		slots, err := r.scheduling.ListAvailableSlots(ctx, orgID, schedulingdomain.SlotQuery{
			BranchID:   selection.Branch.ID,
			ServiceID:  selection.Service.ID,
			Date:       query.Date.UTC(),
			ResourceID: query.ResourceID,
		})
		if err != nil {
			return nil, mapSchedulingErr(err)
		}
		out := make([]AvailabilitySlot, 0, len(slots))
		for _, slot := range slots {
			out = append(out, AvailabilitySlot{
				StartAt:   slot.StartAt.UTC(),
				EndAt:     slot.EndAt.UTC(),
				Remaining: slot.Remaining,
			})
		}
		return out, nil
	}

	return r.getAvailabilityLegacy(ctx, orgID, query.Date, query.Duration)
}

func (r *Repository) Book(ctx context.Context, orgID uuid.UUID, payload map[string]any) (AppointmentPublic, error) {
	branchID, err := uuidPtrFromPayload(payload, "branch_id")
	if err != nil {
		return AppointmentPublic{}, ErrInvalidInput
	}
	serviceID, err := uuidPtrFromPayload(payload, "service_id")
	if err != nil {
		return AppointmentPublic{}, ErrInvalidInput
	}
	resourceID, err := uuidPtrFromPayload(payload, "resource_id")
	if err != nil {
		return AppointmentPublic{}, ErrInvalidInput
	}
	if selection, ok, err := r.resolveSchedulingSelection(ctx, orgID, branchID, serviceID, resourceID); err != nil {
		return AppointmentPublic{}, err
	} else if ok {
		return r.bookScheduling(ctx, orgID, selection, payload)
	}
	return r.bookLegacy(ctx, orgID, payload)
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

	out := make([]AppointmentPublic, 0)
	newRows, err := r.listSchedulingBookingsByPhone(ctx, orgID, phoneDigits, limit)
	if err != nil {
		return nil, err
	}
	out = append(out, newRows...)

	legacyRows, err := r.listLegacyAppointmentsByPhone(ctx, orgID, phoneDigits, limit)
	if err != nil {
		return nil, err
	}
	out = append(out, legacyRows...)

	sort.Slice(out, func(i, j int) bool {
		return out[i].StartAt.After(out[j].StartAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *Repository) listSchedulingPublicServices(ctx context.Context, orgID uuid.UUID, limit int) ([]PublicService, bool, error) {
	if r.scheduling == nil {
		return nil, false, nil
	}
	services, err := r.scheduling.ListServices(ctx, orgID)
	if err != nil {
		return nil, false, err
	}
	active := make([]PublicService, 0, len(services))
	for _, service := range services {
		if !service.Active {
			continue
		}
		unit := "appointment"
		if service.FulfillmentMode == schedulingdomain.FulfillmentModeQueue {
			unit = "ticket"
		}
		active = append(active, PublicService{
			ID:          service.ID,
			Name:        service.Name,
			Type:        string(service.FulfillmentMode),
			Description: service.Description,
			Unit:        unit,
			Price:       0,
			Currency:    "",
		})
	}
	if len(active) == 0 {
		return nil, false, nil
	}
	sort.Slice(active, func(i, j int) bool { return strings.ToLower(active[i].Name) < strings.ToLower(active[j].Name) })
	if len(active) > limit {
		active = active[:limit]
	}
	return active, true, nil
}

func (r *Repository) resolveSchedulingSelection(ctx context.Context, orgID uuid.UUID, branchID, serviceID, resourceID *uuid.UUID) (schedulingSelection, bool, error) {
	if r.scheduling == nil {
		return schedulingSelection{}, false, nil
	}
	branches, err := r.scheduling.ListBranches(ctx, orgID)
	if err != nil {
		return schedulingSelection{}, false, err
	}
	activeBranches := make([]schedulingdomain.Branch, 0, len(branches))
	for _, branch := range branches {
		if branch.Active {
			activeBranches = append(activeBranches, branch)
		}
	}
	services, err := r.scheduling.ListServices(ctx, orgID)
	if err != nil {
		return schedulingSelection{}, false, err
	}
	activeServices := make([]schedulingdomain.Service, 0, len(services))
	for _, service := range services {
		if !service.Active {
			continue
		}
		if service.FulfillmentMode == schedulingdomain.FulfillmentModeQueue {
			continue
		}
		activeServices = append(activeServices, service)
	}
	if len(activeBranches) == 0 || len(activeServices) == 0 {
		return schedulingSelection{}, false, nil
	}

	branch, err := chooseBranch(activeBranches, branchID)
	if err != nil {
		return schedulingSelection{}, true, err
	}
	service, err := chooseService(activeServices, serviceID)
	if err != nil {
		return schedulingSelection{}, true, err
	}

	var resource *schedulingdomain.Resource
	if resourceID != nil && *resourceID != uuid.Nil {
		resources, err := r.scheduling.ListResources(ctx, orgID, &branch.ID)
		if err != nil {
			return schedulingSelection{}, true, err
		}
		found := false
		for _, candidate := range resources {
			if candidate.ID == *resourceID && candidate.Active {
				tmp := candidate
				resource = &tmp
				found = true
				break
			}
		}
		if !found {
			return schedulingSelection{}, true, ErrInvalidInput
		}
	}

	return schedulingSelection{
		Branch:   branch,
		Service:  service,
		Resource: resource,
	}, true, nil
}

func chooseBranch(branches []schedulingdomain.Branch, branchID *uuid.UUID) (schedulingdomain.Branch, error) {
	if branchID != nil && *branchID != uuid.Nil {
		for _, branch := range branches {
			if branch.ID == *branchID {
				return branch, nil
			}
		}
		return schedulingdomain.Branch{}, ErrInvalidInput
	}
	if len(branches) == 1 {
		return branches[0], nil
	}
	return schedulingdomain.Branch{}, ErrInvalidInput
}

func chooseService(services []schedulingdomain.Service, serviceID *uuid.UUID) (schedulingdomain.Service, error) {
	if serviceID != nil && *serviceID != uuid.Nil {
		for _, service := range services {
			if service.ID == *serviceID {
				return service, nil
			}
		}
		return schedulingdomain.Service{}, ErrInvalidInput
	}
	if len(services) == 1 {
		return services[0], nil
	}
	return schedulingdomain.Service{}, ErrInvalidInput
}

func (r *Repository) bookScheduling(ctx context.Context, orgID uuid.UUID, selection schedulingSelection, payload map[string]any) (AppointmentPublic, error) {
	startAt, err := timeValueFromPayload(payload, "start_at")
	if err != nil || startAt.IsZero() {
		return AppointmentPublic{}, ErrInvalidInput
	}
	customerName := firstStringFromPayload(payload, "party_name", "customer_name")
	customerPhone := firstStringFromPayload(payload, "party_phone", "customer_phone")
	if strings.TrimSpace(customerName) == "" || strings.TrimSpace(customerPhone) == "" {
		return AppointmentPublic{}, ErrInvalidInput
	}
	partyID, err := uuidPtrFromPayload(payload, "party_id")
	if err != nil {
		return AppointmentPublic{}, ErrInvalidInput
	}
	idempotencyKey := firstStringFromPayload(payload, "idempotency_key")
	notes := firstStringFromPayload(payload, "notes")
	customTitle := firstStringFromPayload(payload, "title")
	if strings.TrimSpace(customTitle) != "" {
		payload = cloneMap(payload)
		metadata := ensureMap(payload["metadata"])
		metadata["public_title"] = customTitle
		payload["metadata"] = metadata
	}
	holdUntil, err := optionalTimeValueFromPayload(payload, "hold_until")
	if err != nil {
		return AppointmentPublic{}, ErrInvalidInput
	}
	source := firstStringFromPayload(payload, "source")
	if strings.TrimSpace(source) == "" {
		source = string(schedulingdomain.BookingSourcePublicWeb)
	}
	metadata := ensureMap(payload["metadata"])
	input := schedulingdomain.CreateBookingInput{
		BranchID:       selection.Branch.ID,
		ServiceID:      selection.Service.ID,
		PartyID:        partyID,
		CustomerName:   customerName,
		CustomerPhone:  customerPhone,
		StartAt:        startAt.UTC(),
		Source:         schedulingdomain.BookingSource(source),
		IdempotencyKey: idempotencyKey,
		HoldUntil:      holdUntil,
		Notes:          notes,
		Metadata:       metadata,
	}
	if selection.Resource != nil {
		input.ResourceID = &selection.Resource.ID
	}
	booking, err := r.scheduling.CreateBooking(ctx, orgID, "public-api", input)
	if err != nil {
		return AppointmentPublic{}, mapSchedulingErr(err)
	}
	title := strings.TrimSpace(customTitle)
	if title == "" {
		title = selection.Service.Name
	}
	return AppointmentPublic{
		ID:            booking.ID,
		CustomerName:  booking.CustomerName,
		CustomerPhone: booking.CustomerPhone,
		Title:         title,
		Status:        string(booking.Status),
		StartAt:       booking.StartAt.UTC(),
		EndAt:         booking.EndAt.UTC(),
		Duration:      int(booking.EndAt.Sub(booking.StartAt).Minutes()),
	}, nil
}

func (r *Repository) getAvailabilityLegacy(ctx context.Context, orgID uuid.UUID, day time.Time, duration int) ([]AvailabilitySlot, error) {
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

func (r *Repository) bookLegacy(ctx context.Context, orgID uuid.UUID, payload map[string]any) (AppointmentPublic, error) {
	name := firstStringFromPayload(payload, "party_name", "customer_name")
	phone := firstStringFromPayload(payload, "party_phone", "customer_phone")
	title := firstStringFromPayload(payload, "title")
	startAt, err := timeValueFromPayload(payload, "start_at")
	if err != nil {
		return AppointmentPublic{}, ErrInvalidInput
	}
	duration := intValueFromPayload(payload, "duration")
	if strings.TrimSpace(name) == "" || strings.TrimSpace(phone) == "" || strings.TrimSpace(title) == "" {
		return AppointmentPublic{}, ErrInvalidInput
	}
	if duration <= 0 {
		duration = 60
	}
	if duration > 720 {
		return AppointmentPublic{}, ErrInvalidInput
	}

	maxPerSlot := 1
	if v, err := r.findMaxPerSlot(ctx, orgID, startAt); err != nil {
		return AppointmentPublic{}, err
	} else if v > 0 {
		maxPerSlot = v
	}

	endAt := startAt.Add(time.Duration(duration) * time.Minute)
	overlaps, err := r.countOverlaps(ctx, orgID, startAt, endAt)
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
		StartAt:       startAt.UTC(),
		EndAt:         endAt.UTC(),
		Duration:      duration,
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

func (r *Repository) listSchedulingBookingsByPhone(ctx context.Context, orgID uuid.UUID, phoneDigits string, limit int) ([]AppointmentPublic, error) {
	var rows []AppointmentPublic
	err := r.db.WithContext(ctx).
		Table("scheduling_bookings tb").
		Select(`
			tb.id,
			tb.customer_name as party_name,
			tb.customer_phone as party_phone,
			COALESCE(ts.name, tb.reference, 'Turno') as title,
			tb.status,
			tb.start_at,
			tb.end_at,
			EXTRACT(EPOCH FROM (tb.end_at - tb.start_at))::int / 60 as duration
		`).
		Joins("LEFT JOIN scheduling_services ts ON ts.id = tb.service_id").
		Where("tb.org_id = ? AND regexp_replace(tb.customer_phone, '[^0-9]', '', 'g') = ?", orgID, phoneDigits).
		Order("tb.start_at DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "scheduling_bookings") {
			return nil, nil
		}
		return nil, err
	}
	return rows, nil
}

func (r *Repository) listLegacyAppointmentsByPhone(ctx context.Context, orgID uuid.UUID, phoneDigits string, limit int) ([]AppointmentPublic, error) {
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

func mapSchedulingErr(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "not found"):
		return ErrInvalidInput
	case strings.Contains(msg, "validation"):
		return ErrInvalidInput
	case strings.Contains(msg, "required"):
		return ErrInvalidInput
	case strings.Contains(msg, "invalid"):
		return ErrInvalidInput
	case strings.Contains(msg, "slot not available"):
		return ErrSlotUnavailable
	case strings.Contains(msg, "conflict"):
		return ErrSlotUnavailable
	default:
		return err
	}
}

func uuidPtrFromPayload(payload map[string]any, key string) (*uuid.UUID, error) {
	value := firstStringFromPayload(payload, key)
	if value == "" {
		return nil, nil
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func firstStringFromPayload(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := payload[key]
		if !ok || raw == nil {
			continue
		}
		switch value := raw.(type) {
		case string:
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		case fmt.Stringer:
			if trimmed := strings.TrimSpace(value.String()); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func timeValueFromPayload(payload map[string]any, key string) (time.Time, error) {
	raw := firstStringFromPayload(payload, key)
	if raw == "" {
		return time.Time{}, ErrInvalidInput
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func optionalTimeValueFromPayload(payload map[string]any, key string) (*time.Time, error) {
	raw := firstStringFromPayload(payload, key)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func intValueFromPayload(payload map[string]any, key string) int {
	raw, ok := payload[key]
	if !ok || raw == nil {
		return 0
	}
	switch value := raw.(type) {
	case float64:
		return int(value)
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case string:
		if value = strings.TrimSpace(value); value != "" {
			if parsed, err := strconv.Atoi(value); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func ensureMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	if m, ok := v.(map[string]any); ok {
		return cloneMap(m)
	}
	return map[string]any{}
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
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
