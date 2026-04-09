package scheduling

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corescheduling "github.com/devpablocristo/core/scheduling/go"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	schedulingdomain "github.com/devpablocristo/modules/scheduling/go/domain"
	schedulingmodels "github.com/devpablocristo/modules/scheduling/go/repository/models"
)

var bookingStatusesBlocking = []string{
	string(schedulingdomain.BookingStatusHold),
	string(schedulingdomain.BookingStatusPendingConfirmation),
	string(schedulingdomain.BookingStatusConfirmed),
	string(schedulingdomain.BookingStatusCheckedIn),
	string(schedulingdomain.BookingStatusInService),
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ListBranches(ctx context.Context, orgID uuid.UUID) ([]schedulingdomain.Branch, error) {
	var rows []schedulingmodels.BranchModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ?", orgID).
		Order("name ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]schedulingdomain.Branch, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainBranch(row))
	}
	return out, nil
}

func (r *Repository) GetBranch(ctx context.Context, orgID, branchID uuid.UUID) (schedulingdomain.Branch, error) {
	var row schedulingmodels.BranchModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ?", orgID, branchID).
		Take(&row).Error; err != nil {
		return schedulingdomain.Branch{}, err
	}
	return toDomainBranch(row), nil
}

func (r *Repository) CreateBranch(ctx context.Context, in schedulingdomain.Branch) (schedulingdomain.Branch, error) {
	now := time.Now().UTC()
	row := schedulingmodels.BranchModel{
		ID:        in.ID,
		OrgID:     in.OrgID,
		Code:      strings.TrimSpace(in.Code),
		Name:      strings.TrimSpace(in.Name),
		Timezone:  strings.TrimSpace(in.Timezone),
		Address:   strings.TrimSpace(in.Address),
		Active:    in.Active,
		Metadata:  mustJSON(in.Metadata),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return schedulingdomain.Branch{}, err
	}
	return toDomainBranch(row), nil
}

func (r *Repository) ListServices(ctx context.Context, orgID uuid.UUID) ([]schedulingdomain.Service, error) {
	var rows []schedulingmodels.ServiceModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ?", orgID).
		Order("name ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	serviceIDs := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		serviceIDs = append(serviceIDs, row.ID)
	}
	resourceMap, err := r.loadServiceResourceMap(ctx, serviceIDs)
	if err != nil {
		return nil, err
	}
	out := make([]schedulingdomain.Service, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainService(row, resourceMap[row.ID]))
	}
	return out, nil
}

func (r *Repository) GetService(ctx context.Context, orgID, serviceID uuid.UUID) (schedulingdomain.Service, error) {
	var row schedulingmodels.ServiceModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ?", orgID, serviceID).
		Take(&row).Error; err != nil {
		return schedulingdomain.Service{}, err
	}
	resourceMap, err := r.loadServiceResourceMap(ctx, []uuid.UUID{serviceID})
	if err != nil {
		return schedulingdomain.Service{}, err
	}
	return toDomainService(row, resourceMap[serviceID]), nil
}

func (r *Repository) CreateService(ctx context.Context, in schedulingdomain.Service) (schedulingdomain.Service, error) {
	var out schedulingdomain.Service
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		row := schedulingmodels.ServiceModel{
			ID:                     in.ID,
			OrgID:                  in.OrgID,
			CommercialServiceID:    in.CommercialServiceID,
			Code:                   strings.TrimSpace(in.Code),
			Name:                   strings.TrimSpace(in.Name),
			Description:            strings.TrimSpace(in.Description),
			FulfillmentMode:        string(in.FulfillmentMode),
			DefaultDurationMinutes: in.DefaultDurationMinutes,
			BufferBeforeMinutes:    in.BufferBeforeMinutes,
			BufferAfterMinutes:     in.BufferAfterMinutes,
			SlotGranularityMinutes: in.SlotGranularityMinutes,
			MaxConcurrentBookings:  in.MaxConcurrentBookings,
			MinCancelNoticeMinutes: in.MinCancelNoticeMinutes,
			AllowWaitlist:          in.AllowWaitlist,
			Active:                 in.Active,
			Metadata:               mustJSON(in.Metadata),
			CreatedAt:              now,
			UpdatedAt:              now,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		for _, resourceID := range dedupeUUIDs(in.ResourceIDs) {
			link := schedulingmodels.ServiceResourceModel{
				ServiceID:  row.ID,
				ResourceID: resourceID,
				CreatedAt:  now,
			}
			if err := tx.Create(&link).Error; err != nil {
				return err
			}
		}
		out = toDomainService(row, dedupeUUIDs(in.ResourceIDs))
		return nil
	})
	return out, err
}

func (r *Repository) ListResources(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]schedulingdomain.Resource, error) {
	q := r.db.WithContext(ctx).Model(&schedulingmodels.ResourceModel{}).Where("org_id = ?", orgID)
	if branchID != nil && *branchID != uuid.Nil {
		q = q.Where("branch_id = ?", *branchID)
	}
	var rows []schedulingmodels.ResourceModel
	if err := q.Order("name ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]schedulingdomain.Resource, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainResource(row))
	}
	return out, nil
}

func (r *Repository) GetResource(ctx context.Context, orgID, resourceID uuid.UUID) (schedulingdomain.Resource, error) {
	var row schedulingmodels.ResourceModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ?", orgID, resourceID).
		Take(&row).Error; err != nil {
		return schedulingdomain.Resource{}, err
	}
	return toDomainResource(row), nil
}

func (r *Repository) ListServiceResources(ctx context.Context, orgID, branchID, serviceID uuid.UUID, selected *uuid.UUID) ([]schedulingdomain.Resource, error) {
	q := r.db.WithContext(ctx).
		Table("scheduling_resources r").
		Select("r.*").
		Joins("JOIN scheduling_service_resources sr ON sr.resource_id = r.id").
		Where("r.org_id = ? AND r.branch_id = ? AND sr.service_id = ? AND r.active = TRUE", orgID, branchID, serviceID)
	if selected != nil && *selected != uuid.Nil {
		q = q.Where("r.id = ?", *selected)
	}
	var rows []schedulingmodels.ResourceModel
	if err := q.Order("r.name ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]schedulingdomain.Resource, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainResource(row))
	}
	return out, nil
}

func (r *Repository) CreateResource(ctx context.Context, in schedulingdomain.Resource) (schedulingdomain.Resource, error) {
	now := time.Now().UTC()
	row := schedulingmodels.ResourceModel{
		ID:        in.ID,
		OrgID:     in.OrgID,
		BranchID:  in.BranchID,
		Code:      strings.TrimSpace(in.Code),
		Name:      strings.TrimSpace(in.Name),
		Kind:      string(in.Kind),
		Capacity:  in.Capacity,
		Timezone:  strings.TrimSpace(in.Timezone),
		Active:    in.Active,
		Metadata:  mustJSON(in.Metadata),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return schedulingdomain.Resource{}, err
	}
	return toDomainResource(row), nil
}

func (r *Repository) ListAvailabilityRules(ctx context.Context, orgID uuid.UUID, branchID, resourceID *uuid.UUID) ([]schedulingdomain.AvailabilityRule, error) {
	q := r.db.WithContext(ctx).Model(&schedulingmodels.AvailabilityRuleModel{}).Where("org_id = ?", orgID)
	if branchID != nil && *branchID != uuid.Nil {
		q = q.Where("branch_id = ?", *branchID)
	}
	if resourceID != nil && *resourceID != uuid.Nil {
		q = q.Where("resource_id = ?", *resourceID)
	}
	var rows []schedulingmodels.AvailabilityRuleModel
	if err := q.Order("branch_id ASC, resource_id ASC, weekday ASC, start_time ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]schedulingdomain.AvailabilityRule, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainAvailabilityRule(row))
	}
	return out, nil
}

func (r *Repository) ListApplicableAvailabilityRules(ctx context.Context, orgID, branchID uuid.UUID, resourceID *uuid.UUID, day time.Time) ([]schedulingdomain.AvailabilityRule, error) {
	dayDate := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	q := r.db.WithContext(ctx).
		Model(&schedulingmodels.AvailabilityRuleModel{}).
		Where("org_id = ? AND branch_id = ? AND active = TRUE AND weekday = ?", orgID, branchID, int(day.Weekday())).
		Where("(valid_from IS NULL OR valid_from <= ?) AND (valid_until IS NULL OR valid_until >= ?)", dayDate, dayDate)
	if resourceID != nil && *resourceID != uuid.Nil {
		q = q.Where("(kind = ? AND resource_id IS NULL) OR (kind = ? AND resource_id = ?)", string(schedulingdomain.AvailabilityRuleKindBranch), string(schedulingdomain.AvailabilityRuleKindResource), *resourceID)
	} else {
		q = q.Where("kind = ? AND resource_id IS NULL", string(schedulingdomain.AvailabilityRuleKindBranch))
	}
	var rows []schedulingmodels.AvailabilityRuleModel
	if err := q.Order("kind ASC, start_time ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]schedulingdomain.AvailabilityRule, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainAvailabilityRule(row))
	}
	return out, nil
}

func (r *Repository) CreateAvailabilityRule(ctx context.Context, in schedulingdomain.AvailabilityRule) (schedulingdomain.AvailabilityRule, error) {
	// Validate clock format up front; the actual storage is the canonical
	// "HH:MM" string so postgres `time` columns accept it without conversion.
	if _, err := corescheduling.ParseClock(in.StartTime); err != nil {
		return schedulingdomain.AvailabilityRule{}, err
	}
	if _, err := corescheduling.ParseClock(in.EndTime); err != nil {
		return schedulingdomain.AvailabilityRule{}, err
	}
	now := time.Now().UTC()
	row := schedulingmodels.AvailabilityRuleModel{
		ID:                     in.ID,
		OrgID:                  in.OrgID,
		BranchID:               in.BranchID,
		ResourceID:             in.ResourceID,
		Kind:                   string(in.Kind),
		Weekday:                in.Weekday,
		StartTime:              in.StartTime,
		EndTime:                in.EndTime,
		SlotGranularityMinutes: in.SlotGranularityMinutes,
		ValidFrom:              in.ValidFrom,
		ValidUntil:             in.ValidUntil,
		Active:                 in.Active,
		Metadata:               mustJSON(in.Metadata),
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return schedulingdomain.AvailabilityRule{}, err
	}
	return toDomainAvailabilityRule(row), nil
}

func (r *Repository) ListBlockedRanges(ctx context.Context, orgID uuid.UUID, branchID, resourceID *uuid.UUID, day *time.Time) ([]schedulingdomain.BlockedRange, error) {
	q := r.db.WithContext(ctx).Model(&schedulingmodels.BlockedRangeModel{}).Where("org_id = ?", orgID)
	if branchID != nil && *branchID != uuid.Nil {
		q = q.Where("branch_id = ?", *branchID)
	}
	if resourceID != nil && *resourceID != uuid.Nil {
		q = q.Where("resource_id = ?", *resourceID)
	}
	if day != nil && !day.IsZero() {
		from := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
		to := from.Add(24 * time.Hour)
		q = q.Where("start_at < ? AND end_at > ?", to, from)
	}
	var rows []schedulingmodels.BlockedRangeModel
	if err := q.Order("start_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]schedulingdomain.BlockedRange, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainBlockedRange(row))
	}
	return out, nil
}

func (r *Repository) ListBlockedRangesBetween(ctx context.Context, orgID, branchID uuid.UUID, resourceID *uuid.UUID, startAt, endAt time.Time) ([]schedulingdomain.BlockedRange, error) {
	q := r.db.WithContext(ctx).
		Model(&schedulingmodels.BlockedRangeModel{}).
		Where("org_id = ? AND branch_id = ? AND start_at < ? AND end_at > ?", orgID, branchID, endAt.UTC(), startAt.UTC())
	if resourceID != nil && *resourceID != uuid.Nil {
		q = q.Where("resource_id IS NULL OR resource_id = ?", *resourceID)
	} else {
		q = q.Where("resource_id IS NULL")
	}
	var rows []schedulingmodels.BlockedRangeModel
	if err := q.Order("start_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]schedulingdomain.BlockedRange, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainBlockedRange(row))
	}
	return out, nil
}

func (r *Repository) CreateBlockedRange(ctx context.Context, in schedulingdomain.BlockedRange) (schedulingdomain.BlockedRange, error) {
	now := time.Now().UTC()
	row := schedulingmodels.BlockedRangeModel{
		ID:         in.ID,
		OrgID:      in.OrgID,
		BranchID:   in.BranchID,
		ResourceID: in.ResourceID,
		Kind:       string(in.Kind),
		Reason:     strings.TrimSpace(in.Reason),
		StartAt:    in.StartAt.UTC(),
		EndAt:      in.EndAt.UTC(),
		AllDay:     in.AllDay,
		CreatedBy:  strings.TrimSpace(in.CreatedBy),
		Metadata:   mustJSON(in.Metadata),
		CreatedAt:  now,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return schedulingdomain.BlockedRange{}, err
	}
	return toDomainBlockedRange(row), nil
}

func (r *Repository) UpdateBlockedRange(ctx context.Context, in schedulingdomain.BlockedRange) (schedulingdomain.BlockedRange, error) {
	updates := map[string]any{
		"branch_id":   in.BranchID,
		"resource_id": in.ResourceID,
		"kind":        string(in.Kind),
		"reason":      strings.TrimSpace(in.Reason),
		"start_at":    in.StartAt.UTC(),
		"end_at":      in.EndAt.UTC(),
		"all_day":     in.AllDay,
		"metadata":    mustJSON(in.Metadata),
	}
	res := r.db.WithContext(ctx).
		Model(&schedulingmodels.BlockedRangeModel{}).
		Where("org_id = ? AND id = ?", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return schedulingdomain.BlockedRange{}, res.Error
	}
	if res.RowsAffected == 0 {
		return schedulingdomain.BlockedRange{}, gorm.ErrRecordNotFound
	}
	var row schedulingmodels.BlockedRangeModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ?", in.OrgID, in.ID).
		Take(&row).Error; err != nil {
		return schedulingdomain.BlockedRange{}, err
	}
	return toDomainBlockedRange(row), nil
}

func (r *Repository) DeleteBlockedRange(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ?", orgID, id).
		Delete(&schedulingmodels.BlockedRangeModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ── calendar events ──────────────────────────────────────────────────────────

func (r *Repository) CreateCalendarEvent(ctx context.Context, in schedulingdomain.CalendarEvent) (schedulingdomain.CalendarEvent, error) {
	now := time.Now().UTC()
	row := schedulingmodels.CalendarEventModel{
		ID:          in.ID,
		OrgID:       in.OrgID,
		BranchID:    in.BranchID,
		ResourceID:  in.ResourceID,
		Title:       strings.TrimSpace(in.Title),
		Description: strings.TrimSpace(in.Description),
		StartAt:     in.StartAt.UTC(),
		EndAt:       in.EndAt.UTC(),
		AllDay:      in.AllDay,
		Status:      string(in.Status),
		Visibility:  string(in.Visibility),
		CreatedBy:   strings.TrimSpace(in.CreatedBy),
		Metadata:    mustJSON(in.Metadata),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return schedulingdomain.CalendarEvent{}, err
	}
	return toDomainCalendarEvent(row), nil
}

func (r *Repository) GetCalendarEvent(ctx context.Context, orgID, id uuid.UUID) (schedulingdomain.CalendarEvent, error) {
	var row schedulingmodels.CalendarEventModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ?", orgID, id).
		Take(&row).Error; err != nil {
		return schedulingdomain.CalendarEvent{}, err
	}
	return toDomainCalendarEvent(row), nil
}

func (r *Repository) ListCalendarEvents(ctx context.Context, orgID uuid.UUID, filter schedulingdomain.ListCalendarEventsFilter) ([]schedulingdomain.CalendarEvent, error) {
	q := r.db.WithContext(ctx).Model(&schedulingmodels.CalendarEventModel{}).Where("org_id = ?", orgID)
	if filter.BranchID != nil && *filter.BranchID != uuid.Nil {
		q = q.Where("branch_id = ?", *filter.BranchID)
	}
	if filter.ResourceID != nil && *filter.ResourceID != uuid.Nil {
		q = q.Where("resource_id = ?", *filter.ResourceID)
	}
	if filter.From != nil && !filter.From.IsZero() {
		q = q.Where("end_at > ?", filter.From.UTC())
	}
	if filter.To != nil && !filter.To.IsZero() {
		q = q.Where("start_at < ?", filter.To.UTC())
	}
	if filter.Status != nil && *filter.Status != "" {
		q = q.Where("status = ?", string(*filter.Status))
	}
	var rows []schedulingmodels.CalendarEventModel
	if err := q.Order("start_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]schedulingdomain.CalendarEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainCalendarEvent(row))
	}
	return out, nil
}

// ListCalendarEventsOccupyingResource devuelve los eventos no cancelados que
// se solapan con [from, to] sobre un recurso concreto. Lo usa el slot picker
// para restar tiempo ocupado por agenda interna.
func (r *Repository) ListCalendarEventsOccupyingResource(ctx context.Context, orgID, branchID, resourceID uuid.UUID, from, to time.Time) ([]schedulingdomain.CalendarEvent, error) {
	q := r.db.WithContext(ctx).
		Model(&schedulingmodels.CalendarEventModel{}).
		Where("org_id = ? AND resource_id = ? AND status <> ?", orgID, resourceID, string(schedulingdomain.CalendarEventStatusCancelled)).
		Where("start_at < ? AND end_at > ?", to.UTC(), from.UTC())
	if branchID != uuid.Nil {
		q = q.Where("branch_id IS NULL OR branch_id = ?", branchID)
	}
	var rows []schedulingmodels.CalendarEventModel
	if err := q.Order("start_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]schedulingdomain.CalendarEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainCalendarEvent(row))
	}
	return out, nil
}

func (r *Repository) UpdateCalendarEvent(ctx context.Context, in schedulingdomain.CalendarEvent) (schedulingdomain.CalendarEvent, error) {
	updates := map[string]any{
		"branch_id":   in.BranchID,
		"resource_id": in.ResourceID,
		"title":       strings.TrimSpace(in.Title),
		"description": strings.TrimSpace(in.Description),
		"start_at":    in.StartAt.UTC(),
		"end_at":      in.EndAt.UTC(),
		"all_day":     in.AllDay,
		"status":      string(in.Status),
		"visibility":  string(in.Visibility),
		"metadata":    mustJSON(in.Metadata),
		"updated_at":  time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).
		Model(&schedulingmodels.CalendarEventModel{}).
		Where("org_id = ? AND id = ?", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return schedulingdomain.CalendarEvent{}, res.Error
	}
	if res.RowsAffected == 0 {
		return schedulingdomain.CalendarEvent{}, gorm.ErrRecordNotFound
	}
	var row schedulingmodels.CalendarEventModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ?", in.OrgID, in.ID).
		Take(&row).Error; err != nil {
		return schedulingdomain.CalendarEvent{}, err
	}
	return toDomainCalendarEvent(row), nil
}

func (r *Repository) DeleteCalendarEvent(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ?", orgID, id).
		Delete(&schedulingmodels.CalendarEventModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) CountBookingOverlaps(ctx context.Context, orgID, resourceID uuid.UUID, occupiesFrom, occupiesUntil time.Time, excludeBookingID *uuid.UUID) (int64, error) {
	q := r.db.WithContext(ctx).
		Model(&schedulingmodels.BookingModel{}).
		Where("org_id = ? AND resource_id = ? AND status IN ?", orgID, resourceID, bookingStatusesBlocking).
		Where("(status <> ? OR hold_expires_at IS NULL OR hold_expires_at >= ?)", string(schedulingdomain.BookingStatusHold), time.Now().UTC()).
		Where("occupies_from < ? AND occupies_until > ?", occupiesUntil.UTC(), occupiesFrom.UTC())
	if excludeBookingID != nil && *excludeBookingID != uuid.Nil {
		q = q.Where("id <> ?", *excludeBookingID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) CreateBookings(ctx context.Context, items []schedulingdomain.Booking) ([]schedulingdomain.Booking, error) {
	out := make([]schedulingdomain.Booking, 0, len(items))
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			created, err := r.createBookingTx(tx, item)
			if err != nil {
				return err
			}
			out = append(out, created)
		}
		return nil
	})
	return out, err
}

func (r *Repository) CreateBooking(ctx context.Context, in schedulingdomain.Booking) (schedulingdomain.Booking, error) {
	out, err := r.CreateBookings(ctx, []schedulingdomain.Booking{in})
	if err != nil {
		return schedulingdomain.Booking{}, err
	}
	if len(out) == 0 {
		return schedulingdomain.Booking{}, nil
	}
	return out[0], nil
}

func (r *Repository) createBookingTx(tx *gorm.DB, in schedulingdomain.Booking) (schedulingdomain.Booking, error) {
	if strings.TrimSpace(in.IdempotencyKey) != "" {
		var existing schedulingmodels.BookingModel
		err := tx.Where("org_id = ? AND idempotency_key = ?", in.OrgID, strings.TrimSpace(in.IdempotencyKey)).Take(&existing).Error
		if err == nil {
			return toDomainBooking(existing), nil
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return schedulingdomain.Booking{}, err
		}
	}
	if err := expireOverdueHoldsTx(tx, in.OrgID, in.ResourceID, in.OccupiesFrom, in.OccupiesUntil); err != nil {
		return schedulingdomain.Booking{}, err
	}
	now := time.Now().UTC()
	row := schedulingmodels.BookingModel{
		ID:             in.ID,
		OrgID:          in.OrgID,
		BranchID:       in.BranchID,
		ServiceID:      in.ServiceID,
		ResourceID:     in.ResourceID,
		PartyID:        in.PartyID,
		Reference:      strings.TrimSpace(in.Reference),
		CustomerName:   strings.TrimSpace(in.CustomerName),
		CustomerPhone:  strings.TrimSpace(in.CustomerPhone),
		CustomerEmail:  strings.TrimSpace(in.CustomerEmail),
		Status:         string(in.Status),
		Source:         string(in.Source),
		IdempotencyKey: nullableTrimmed(in.IdempotencyKey),
		StartAt:        in.StartAt.UTC(),
		EndAt:          in.EndAt.UTC(),
		OccupiesFrom:   in.OccupiesFrom.UTC(),
		OccupiesUntil:  in.OccupiesUntil.UTC(),
		HoldExpiresAt:  in.HoldExpiresAt,
		Notes:          strings.TrimSpace(in.Notes),
		Metadata:       mustJSON(in.Metadata),
		CreatedBy:      strings.TrimSpace(in.CreatedBy),
		ConfirmedAt:    in.ConfirmedAt,
		CancelledAt:    in.CancelledAt,
		ReminderSentAt: in.ReminderSentAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := tx.Create(&row).Error; err != nil {
		return schedulingdomain.Booking{}, err
	}
	return toDomainBooking(row), nil
}

func (r *Repository) GetBookingByID(ctx context.Context, orgID, bookingID uuid.UUID) (schedulingdomain.Booking, error) {
	var row schedulingmodels.BookingModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ?", orgID, bookingID).
		Take(&row).Error; err != nil {
		return schedulingdomain.Booking{}, err
	}
	return toDomainBooking(row), nil
}

func (r *Repository) ListBookings(ctx context.Context, orgID uuid.UUID, filter schedulingdomain.ListBookingsFilter) ([]schedulingdomain.Booking, error) {
	q := r.db.WithContext(ctx).Model(&schedulingmodels.BookingModel{}).Where("org_id = ?", orgID)
	if filter.BranchID != nil && *filter.BranchID != uuid.Nil {
		q = q.Where("branch_id = ?", *filter.BranchID)
	}
	if filter.Date != nil && !filter.Date.IsZero() {
		from := time.Date(filter.Date.Year(), filter.Date.Month(), filter.Date.Day(), 0, 0, 0, 0, time.UTC)
		to := from.Add(24 * time.Hour)
		q = q.Where("start_at >= ? AND start_at < ?", from, to)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		q = q.Where("status = ?", status)
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	var rows []schedulingmodels.BookingModel
	if err := q.Order("start_at ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]schedulingdomain.Booking, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainBooking(row))
	}
	return out, nil
}

func (r *Repository) ListBookingsByPhone(ctx context.Context, orgID uuid.UUID, phoneDigits string, limit int) ([]schedulingdomain.Booking, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	var rows []schedulingmodels.BookingModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND regexp_replace(customer_phone, '[^0-9]', '', 'g') = ?", orgID, strings.TrimSpace(phoneDigits)).
		Order("start_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]schedulingdomain.Booking, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainBooking(row))
	}
	return out, nil
}

func (r *Repository) UpdateBookingStatus(ctx context.Context, orgID, bookingID uuid.UUID, status schedulingdomain.BookingStatus, confirmedAt, cancelledAt *time.Time, notes string) (schedulingdomain.Booking, error) {
	now := time.Now().UTC()
	updates := map[string]any{
		"status":     string(status),
		"updated_at": now,
	}
	if status != schedulingdomain.BookingStatusHold {
		updates["hold_expires_at"] = nil
	}
	if confirmedAt != nil {
		updates["confirmed_at"] = confirmedAt.UTC()
	}
	if cancelledAt != nil {
		updates["cancelled_at"] = cancelledAt.UTC()
	}
	if strings.TrimSpace(notes) != "" {
		updates["notes"] = strings.TrimSpace(notes)
	}
	if err := r.db.WithContext(ctx).
		Model(&schedulingmodels.BookingModel{}).
		Where("org_id = ? AND id = ?", orgID, bookingID).
		Updates(updates).Error; err != nil {
		return schedulingdomain.Booking{}, err
	}
	return r.GetBookingByID(ctx, orgID, bookingID)
}

func (r *Repository) MarkBookingReminderSent(ctx context.Context, orgID, bookingID uuid.UUID, sentAt time.Time) (schedulingdomain.Booking, error) {
	if err := r.db.WithContext(ctx).
		Model(&schedulingmodels.BookingModel{}).
		Where("org_id = ? AND id = ?", orgID, bookingID).
		Updates(map[string]any{
			"reminder_sent_at": sentAt.UTC(),
			"updated_at":       time.Now().UTC(),
		}).Error; err != nil {
		return schedulingdomain.Booking{}, err
	}
	return r.GetBookingByID(ctx, orgID, bookingID)
}

func (r *Repository) ExpireOverdueHolds(ctx context.Context, limit int) ([]schedulingdomain.Booking, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	var rows []schedulingmodels.BookingModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND hold_expires_at IS NOT NULL AND hold_expires_at < ?", string(schedulingdomain.BookingStatusHold), time.Now().UTC()).
			Order("hold_expires_at ASC").
			Limit(limit).
			Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]uuid.UUID, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		return tx.Model(&schedulingmodels.BookingModel{}).
			Where("id IN ?", ids).
			Updates(map[string]any{
				"status":          string(schedulingdomain.BookingStatusExpired),
				"hold_expires_at": nil,
				"updated_at":      time.Now().UTC(),
			}).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]schedulingdomain.Booking, 0, len(rows))
	for _, row := range rows {
		row.Status = string(schedulingdomain.BookingStatusExpired)
		row.HoldExpiresAt = nil
		row.UpdatedAt = time.Now().UTC()
		out = append(out, toDomainBooking(row))
	}
	return out, nil
}

func (r *Repository) RescheduleBooking(ctx context.Context, in schedulingdomain.Booking) (schedulingdomain.Booking, error) {
	var out schedulingdomain.Booking
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := expireOverdueHoldsTx(tx, in.OrgID, in.ResourceID, in.OccupiesFrom, in.OccupiesUntil); err != nil {
			return err
		}
		updates := map[string]any{
			"branch_id":      in.BranchID,
			"resource_id":    in.ResourceID,
			"start_at":       in.StartAt.UTC(),
			"end_at":         in.EndAt.UTC(),
			"occupies_from":  in.OccupiesFrom.UTC(),
			"occupies_until": in.OccupiesUntil.UTC(),
			"updated_at":     time.Now().UTC(),
		}
		if err := tx.Model(&schedulingmodels.BookingModel{}).
			Where("org_id = ? AND id = ?", in.OrgID, in.ID).
			Updates(updates).Error; err != nil {
			return err
		}
		var row schedulingmodels.BookingModel
		if err := tx.Where("org_id = ? AND id = ?", in.OrgID, in.ID).Take(&row).Error; err != nil {
			return err
		}
		out = toDomainBooking(row)
		return nil
	})
	return out, err
}

func (r *Repository) ListQueues(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]schedulingdomain.Queue, error) {
	q := r.db.WithContext(ctx).Model(&schedulingmodels.QueueModel{}).Where("org_id = ?", orgID)
	if branchID != nil && *branchID != uuid.Nil {
		q = q.Where("branch_id = ?", *branchID)
	}
	var rows []schedulingmodels.QueueModel
	if err := q.Order("name ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]schedulingdomain.Queue, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainQueue(row))
	}
	return out, nil
}

func (r *Repository) GetQueueByID(ctx context.Context, orgID, queueID uuid.UUID) (schedulingdomain.Queue, error) {
	var row schedulingmodels.QueueModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ?", orgID, queueID).
		Take(&row).Error; err != nil {
		return schedulingdomain.Queue{}, err
	}
	return toDomainQueue(row), nil
}

func (r *Repository) UpdateQueueStatus(ctx context.Context, orgID, queueID uuid.UUID, status schedulingdomain.QueueStatus) (schedulingdomain.Queue, error) {
	if err := r.db.WithContext(ctx).
		Model(&schedulingmodels.QueueModel{}).
		Where("org_id = ? AND id = ?", orgID, queueID).
		Updates(map[string]any{
			"status":     string(status),
			"updated_at": time.Now().UTC(),
		}).Error; err != nil {
		return schedulingdomain.Queue{}, err
	}
	return r.GetQueueByID(ctx, orgID, queueID)
}

func (r *Repository) CreateQueue(ctx context.Context, in schedulingdomain.Queue) (schedulingdomain.Queue, error) {
	now := time.Now().UTC()
	row := schedulingmodels.QueueModel{
		ID:               in.ID,
		OrgID:            in.OrgID,
		BranchID:         in.BranchID,
		ServiceID:        in.ServiceID,
		Code:             strings.TrimSpace(in.Code),
		Name:             strings.TrimSpace(in.Name),
		Status:           string(in.Status),
		Strategy:         string(in.Strategy),
		TicketPrefix:     strings.TrimSpace(in.TicketPrefix),
		LastIssuedNumber: in.LastIssuedNumber,
		AvgServiceSecond: in.AvgServiceSecond,
		AllowRemoteJoin:  in.AllowRemoteJoin,
		Metadata:         mustJSON(in.Metadata),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return schedulingdomain.Queue{}, err
	}
	return toDomainQueue(row), nil
}

func (r *Repository) CreateQueueTicket(ctx context.Context, in schedulingdomain.QueueTicket) (schedulingdomain.QueueTicket, error) {
	var out schedulingdomain.QueueTicket
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if strings.TrimSpace(in.IdempotencyKey) != "" {
			var existing schedulingmodels.QueueTicketModel
			err := tx.Where("org_id = ? AND idempotency_key = ?", in.OrgID, strings.TrimSpace(in.IdempotencyKey)).Take(&existing).Error
			if err == nil {
				out = toDomainQueueTicket(existing)
				return nil
			}
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}
		}
		var queue schedulingmodels.QueueModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("org_id = ? AND id = ?", in.OrgID, in.QueueID).
			Take(&queue).Error; err != nil {
			return err
		}
		if queue.Status != string(schedulingdomain.QueueStatusActive) {
			return errQueueInactive
		}
		next := queue.LastIssuedNumber + 1
		now := time.Now().UTC()
		row := schedulingmodels.QueueTicketModel{
			ID:                in.ID,
			OrgID:             in.OrgID,
			QueueID:           in.QueueID,
			BranchID:          in.BranchID,
			ServiceID:         in.ServiceID,
			PartyID:           in.PartyID,
			CustomerName:      strings.TrimSpace(in.CustomerName),
			CustomerPhone:     strings.TrimSpace(in.CustomerPhone),
			CustomerEmail:     strings.TrimSpace(in.CustomerEmail),
			Number:            next,
			DisplayCode:       buildDisplayCode(queue.TicketPrefix, next),
			Status:            string(in.Status),
			Priority:          in.Priority,
			Source:            string(in.Source),
			IdempotencyKey:    strings.TrimSpace(in.IdempotencyKey),
			ServingResourceID: in.ServingResourceID,
			OperatorUserID:    in.OperatorUserID,
			RequestedAt:       now,
			Notes:             strings.TrimSpace(in.Notes),
			Metadata:          mustJSON(in.Metadata),
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Model(&schedulingmodels.QueueModel{}).
			Where("id = ?", queue.ID).
			Updates(map[string]any{"last_issued_number": next, "updated_at": now}).Error; err != nil {
			return err
		}
		out = toDomainQueueTicket(row)
		return nil
	})
	return out, err
}

func (r *Repository) GetQueueTicket(ctx context.Context, orgID, queueID, ticketID uuid.UUID) (schedulingdomain.QueueTicket, error) {
	var row schedulingmodels.QueueTicketModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND queue_id = ? AND id = ?", orgID, queueID, ticketID).
		Take(&row).Error; err != nil {
		return schedulingdomain.QueueTicket{}, err
	}
	return toDomainQueueTicket(row), nil
}

func (r *Repository) GetQueueTicketPosition(ctx context.Context, orgID, queueID, ticketID uuid.UUID) (schedulingdomain.QueuePosition, error) {
	ticket, err := r.GetQueueTicket(ctx, orgID, queueID, ticketID)
	if err != nil {
		return schedulingdomain.QueuePosition{}, err
	}
	queue, err := r.GetQueueByID(ctx, orgID, queueID)
	if err != nil {
		return schedulingdomain.QueuePosition{}, err
	}
	if ticket.Status != schedulingdomain.QueueTicketStatusWaiting {
		return schedulingdomain.QueuePosition{
			TicketID:         ticket.ID,
			QueueID:          ticket.QueueID,
			Status:           ticket.Status,
			Position:         0,
			EstimatedWaitSec: 0,
		}, nil
	}
	var ahead int64
	err = r.db.WithContext(ctx).
		Model(&schedulingmodels.QueueTicketModel{}).
		Where("org_id = ? AND queue_id = ? AND status = ?", orgID, queueID, string(schedulingdomain.QueueTicketStatusWaiting)).
		Where("(priority < ?) OR (priority = ? AND requested_at < ?) OR (priority = ? AND requested_at = ? AND number < ?)", ticket.Priority, ticket.Priority, ticket.RequestedAt, ticket.Priority, ticket.RequestedAt, ticket.Number).
		Count(&ahead).Error
	if err != nil {
		return schedulingdomain.QueuePosition{}, err
	}
	return schedulingdomain.QueuePosition{
		TicketID:         ticket.ID,
		QueueID:          ticket.QueueID,
		Status:           ticket.Status,
		Position:         int(ahead) + 1,
		EstimatedWaitSec: int(ahead) * queue.AvgServiceSecond,
	}, nil
}

func (r *Repository) CallNextTicket(ctx context.Context, orgID, queueID uuid.UUID, servingResourceID, operatorUserID *uuid.UUID) (schedulingdomain.QueueTicket, error) {
	var out schedulingdomain.QueueTicket
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var queue schedulingmodels.QueueModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("org_id = ? AND id = ?", orgID, queueID).
			Take(&queue).Error; err != nil {
			return err
		}
		if queue.Status != string(schedulingdomain.QueueStatusActive) {
			return errQueueInactive
		}
		var ticket schedulingmodels.QueueTicketModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("org_id = ? AND queue_id = ? AND status = ?", orgID, queueID, string(schedulingdomain.QueueTicketStatusWaiting)).
			Order("priority ASC, requested_at ASC, number ASC").
			First(&ticket).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		updates := map[string]any{
			"status":     string(schedulingdomain.QueueTicketStatusCalled),
			"called_at":  now,
			"updated_at": now,
		}
		if servingResourceID != nil && *servingResourceID != uuid.Nil {
			updates["serving_resource_id"] = *servingResourceID
		}
		if operatorUserID != nil && *operatorUserID != uuid.Nil {
			updates["operator_user_id"] = *operatorUserID
		}
		if err := tx.Model(&schedulingmodels.QueueTicketModel{}).
			Where("id = ?", ticket.ID).
			Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", ticket.ID).Take(&ticket).Error; err != nil {
			return err
		}
		out = toDomainQueueTicket(ticket)
		return nil
	})
	return out, err
}

func (r *Repository) MarkQueueTicketServing(ctx context.Context, orgID, queueID, ticketID uuid.UUID, servingResourceID, operatorUserID *uuid.UUID) (schedulingdomain.QueueTicket, error) {
	now := time.Now().UTC()
	updates := map[string]any{
		"status":     string(schedulingdomain.QueueTicketStatusServing),
		"started_at": now,
		"updated_at": now,
	}
	if servingResourceID != nil && *servingResourceID != uuid.Nil {
		updates["serving_resource_id"] = *servingResourceID
	}
	if operatorUserID != nil && *operatorUserID != uuid.Nil {
		updates["operator_user_id"] = *operatorUserID
	}
	if err := r.db.WithContext(ctx).
		Model(&schedulingmodels.QueueTicketModel{}).
		Where("org_id = ? AND queue_id = ? AND id = ?", orgID, queueID, ticketID).
		Updates(updates).Error; err != nil {
		return schedulingdomain.QueueTicket{}, err
	}
	return r.GetQueueTicket(ctx, orgID, queueID, ticketID)
}

func (r *Repository) UpdateQueueTicketStatus(ctx context.Context, orgID, queueID, ticketID uuid.UUID, status schedulingdomain.QueueTicketStatus, servingResourceID, operatorUserID *uuid.UUID) (schedulingdomain.QueueTicket, error) {
	now := time.Now().UTC()
	updates := map[string]any{
		"status":     string(status),
		"updated_at": now,
	}
	switch status {
	case schedulingdomain.QueueTicketStatusNoShow, schedulingdomain.QueueTicketStatusCancelled:
		updates["cancelled_at"] = now
	case schedulingdomain.QueueTicketStatusCompleted:
		updates["completed_at"] = now
	case schedulingdomain.QueueTicketStatusServing:
		updates["started_at"] = now
	}
	if servingResourceID != nil && *servingResourceID != uuid.Nil {
		updates["serving_resource_id"] = *servingResourceID
	}
	if operatorUserID != nil && *operatorUserID != uuid.Nil {
		updates["operator_user_id"] = *operatorUserID
	}
	if err := r.db.WithContext(ctx).
		Model(&schedulingmodels.QueueTicketModel{}).
		Where("org_id = ? AND queue_id = ? AND id = ?", orgID, queueID, ticketID).
		Updates(updates).Error; err != nil {
		return schedulingdomain.QueueTicket{}, err
	}
	return r.GetQueueTicket(ctx, orgID, queueID, ticketID)
}

func (r *Repository) ReassignQueueTicket(ctx context.Context, orgID, queueID, ticketID uuid.UUID, servingResourceID, operatorUserID *uuid.UUID) (schedulingdomain.QueueTicket, error) {
	updates := map[string]any{
		"updated_at": time.Now().UTC(),
	}
	if servingResourceID != nil && *servingResourceID != uuid.Nil {
		updates["serving_resource_id"] = *servingResourceID
	}
	if operatorUserID != nil && *operatorUserID != uuid.Nil {
		updates["operator_user_id"] = *operatorUserID
	}
	if err := r.db.WithContext(ctx).
		Model(&schedulingmodels.QueueTicketModel{}).
		Where("org_id = ? AND queue_id = ? AND id = ?", orgID, queueID, ticketID).
		Updates(updates).Error; err != nil {
		return schedulingdomain.QueueTicket{}, err
	}
	return r.GetQueueTicket(ctx, orgID, queueID, ticketID)
}

func (r *Repository) ReturnQueueTicketToWaiting(ctx context.Context, orgID, queueID, ticketID uuid.UUID) (schedulingdomain.QueueTicket, error) {
	if err := r.db.WithContext(ctx).
		Model(&schedulingmodels.QueueTicketModel{}).
		Where("org_id = ? AND queue_id = ? AND id = ?", orgID, queueID, ticketID).
		Updates(map[string]any{
			"status":              string(schedulingdomain.QueueTicketStatusWaiting),
			"serving_resource_id": nil,
			"operator_user_id":    nil,
			"called_at":           nil,
			"started_at":          nil,
			"updated_at":          time.Now().UTC(),
		}).Error; err != nil {
		return schedulingdomain.QueueTicket{}, err
	}
	return r.GetQueueTicket(ctx, orgID, queueID, ticketID)
}

func (r *Repository) CreateBookingActionToken(ctx context.Context, in schedulingdomain.BookingActionToken) (schedulingdomain.BookingActionToken, error) {
	row := schedulingmodels.BookingActionTokenModel{
		ID:        in.ID,
		OrgID:     in.OrgID,
		BookingID: in.BookingID,
		Action:    string(in.Action),
		TokenHash: strings.TrimSpace(in.TokenHash),
		ExpiresAt: in.ExpiresAt.UTC(),
		UsedAt:    in.UsedAt,
		VoidedAt:  in.VoidedAt,
		Metadata:  mustJSON(in.Metadata),
		CreatedAt: in.CreatedAt.UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return schedulingdomain.BookingActionToken{}, err
	}
	return toDomainBookingActionToken(row), nil
}

func (r *Repository) GetBookingActionTokenByHash(ctx context.Context, tokenHash string) (schedulingdomain.BookingActionToken, error) {
	var row schedulingmodels.BookingActionTokenModel
	if err := r.db.WithContext(ctx).
		Where("token_hash = ?", strings.TrimSpace(tokenHash)).
		Take(&row).Error; err != nil {
		return schedulingdomain.BookingActionToken{}, err
	}
	return toDomainBookingActionToken(row), nil
}

func (r *Repository) MarkBookingActionTokenUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&schedulingmodels.BookingActionTokenModel{}).
		Where("id = ?", id).
		Updates(map[string]any{"used_at": usedAt.UTC()}).Error
}

func (r *Repository) CreateWaitlistEntry(ctx context.Context, in schedulingdomain.WaitlistEntry) (schedulingdomain.WaitlistEntry, error) {
	var out schedulingdomain.WaitlistEntry
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if strings.TrimSpace(in.IdempotencyKey) != "" {
			var existing schedulingmodels.WaitlistEntryModel
			err := tx.Where("org_id = ? AND idempotency_key = ?", in.OrgID, strings.TrimSpace(in.IdempotencyKey)).Take(&existing).Error
			if err == nil {
				out = toDomainWaitlistEntry(existing)
				return nil
			}
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}
		}
		row := schedulingmodels.WaitlistEntryModel{
			ID:               in.ID,
			OrgID:            in.OrgID,
			BranchID:         in.BranchID,
			ServiceID:        in.ServiceID,
			ResourceID:       in.ResourceID,
			PartyID:          in.PartyID,
			BookingID:        in.BookingID,
			CustomerName:     strings.TrimSpace(in.CustomerName),
			CustomerPhone:    strings.TrimSpace(in.CustomerPhone),
			CustomerEmail:    strings.TrimSpace(in.CustomerEmail),
			RequestedStartAt: in.RequestedStartAt.UTC(),
			Status:           string(in.Status),
			Source:           string(in.Source),
			IdempotencyKey:   strings.TrimSpace(in.IdempotencyKey),
			ExpiresAt:        in.ExpiresAt,
			NotifiedAt:       in.NotifiedAt,
			Notes:            strings.TrimSpace(in.Notes),
			Metadata:         mustJSON(in.Metadata),
			CreatedAt:        in.CreatedAt.UTC(),
			UpdatedAt:        in.UpdatedAt.UTC(),
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		out = toDomainWaitlistEntry(row)
		return nil
	})
	return out, err
}

func (r *Repository) ListWaitlistEntries(ctx context.Context, orgID uuid.UUID, filter schedulingdomain.ListWaitlistFilter) ([]schedulingdomain.WaitlistEntry, error) {
	q := r.db.WithContext(ctx).Model(&schedulingmodels.WaitlistEntryModel{}).Where("org_id = ?", orgID)
	if filter.BranchID != nil && *filter.BranchID != uuid.Nil {
		q = q.Where("branch_id = ?", *filter.BranchID)
	}
	if filter.ServiceID != nil && *filter.ServiceID != uuid.Nil {
		q = q.Where("service_id = ?", *filter.ServiceID)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		q = q.Where("status = ?", status)
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	var rows []schedulingmodels.WaitlistEntryModel
	if err := q.Order("requested_start_at ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]schedulingdomain.WaitlistEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainWaitlistEntry(row))
	}
	return out, nil
}

func (r *Repository) ListPendingWaitlistEntries(ctx context.Context, limit int) ([]schedulingdomain.WaitlistEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	var rows []schedulingmodels.WaitlistEntryModel
	if err := r.db.WithContext(ctx).
		Where("status IN ? AND (expires_at IS NULL OR expires_at >= ?)", []string{string(schedulingdomain.WaitlistStatusPending), string(schedulingdomain.WaitlistStatusNotified)}, time.Now().UTC()).
		Order("requested_start_at ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]schedulingdomain.WaitlistEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainWaitlistEntry(row))
	}
	return out, nil
}

func (r *Repository) UpdateWaitlistEntryStatus(ctx context.Context, orgID, entryID uuid.UUID, status schedulingdomain.WaitlistStatus, expiresAt, notifiedAt *time.Time, bookingID *uuid.UUID, notes string) (schedulingdomain.WaitlistEntry, error) {
	updates := map[string]any{
		"status":     string(status),
		"updated_at": time.Now().UTC(),
	}
	if expiresAt != nil {
		updates["expires_at"] = expiresAt.UTC()
	}
	if notifiedAt != nil {
		updates["notified_at"] = notifiedAt.UTC()
	}
	if bookingID != nil {
		updates["booking_id"] = *bookingID
	}
	if strings.TrimSpace(notes) != "" {
		updates["notes"] = strings.TrimSpace(notes)
	}
	if err := r.db.WithContext(ctx).
		Model(&schedulingmodels.WaitlistEntryModel{}).
		Where("org_id = ? AND id = ?", orgID, entryID).
		Updates(updates).Error; err != nil {
		return schedulingdomain.WaitlistEntry{}, err
	}
	var row schedulingmodels.WaitlistEntryModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ?", orgID, entryID).
		Take(&row).Error; err != nil {
		return schedulingdomain.WaitlistEntry{}, err
	}
	return toDomainWaitlistEntry(row), nil
}

func (r *Repository) DashboardStats(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, day time.Time, timezone string) (schedulingdomain.DashboardStats, error) {
	loc, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		loc = time.UTC
	}
	fromLocal := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	from := fromLocal.UTC()
	to := fromLocal.Add(24 * time.Hour).UTC()

	bookingsQ := r.db.WithContext(ctx).Model(&schedulingmodels.BookingModel{}).Where("org_id = ? AND start_at >= ? AND start_at < ?", orgID, from, to)
	queuesQ := r.db.WithContext(ctx).Model(&schedulingmodels.QueueModel{}).Where("org_id = ?", orgID)
	ticketsQ := r.db.WithContext(ctx).Model(&schedulingmodels.QueueTicketModel{}).Where("org_id = ?", orgID)
	if branchID != nil && *branchID != uuid.Nil {
		bookingsQ = bookingsQ.Where("branch_id = ?", *branchID)
		queuesQ = queuesQ.Where("branch_id = ?", *branchID)
		ticketsQ = ticketsQ.Where("branch_id = ?", *branchID)
	}
	var bookingsToday, confirmedToday, activeQueues, waitingTickets, servingTickets int64
	if err := bookingsQ.Count(&bookingsToday).Error; err != nil {
		return schedulingdomain.DashboardStats{}, err
	}
	if err := bookingsQ.Where("status IN ?", []string{string(schedulingdomain.BookingStatusConfirmed), string(schedulingdomain.BookingStatusCheckedIn), string(schedulingdomain.BookingStatusInService)}).Count(&confirmedToday).Error; err != nil {
		return schedulingdomain.DashboardStats{}, err
	}
	if err := queuesQ.Where("status = ?", string(schedulingdomain.QueueStatusActive)).Count(&activeQueues).Error; err != nil {
		return schedulingdomain.DashboardStats{}, err
	}
	if err := ticketsQ.Where("status = ?", string(schedulingdomain.QueueTicketStatusWaiting)).Count(&waitingTickets).Error; err != nil {
		return schedulingdomain.DashboardStats{}, err
	}
	if err := ticketsQ.Where("status = ?", string(schedulingdomain.QueueTicketStatusServing)).Count(&servingTickets).Error; err != nil {
		return schedulingdomain.DashboardStats{}, err
	}
	return schedulingdomain.DashboardStats{
		Date:                   fromLocal.Format("2006-01-02"),
		Timezone:               timezone,
		BookingsToday:          bookingsToday,
		ConfirmedBookingsToday: confirmedToday,
		ActiveQueues:           activeQueues,
		WaitingTickets:         waitingTickets,
		TicketsInService:       servingTickets,
	}, nil
}

func (r *Repository) ListDayAgenda(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, day time.Time) ([]schedulingdomain.DayAgendaItem, error) {
	items := make([]schedulingdomain.DayAgendaItem, 0)
	bookings, err := r.ListBookings(ctx, orgID, schedulingdomain.ListBookingsFilter{BranchID: branchID, Date: &day, Limit: 500})
	if err != nil {
		return nil, err
	}
	for _, booking := range bookings {
		startAt := booking.StartAt
		endAt := booking.EndAt
		serviceID := booking.ServiceID
		items = append(items, schedulingdomain.DayAgendaItem{
			Type:      "booking",
			ID:        booking.ID,
			BranchID:  booking.BranchID,
			ServiceID: &serviceID,
			StartAt:   &startAt,
			EndAt:     &endAt,
			Status:    string(booking.Status),
			Label:     booking.CustomerName,
			Metadata: map[string]any{
				"reference":   booking.Reference,
				"resource_id": booking.ResourceID.String(),
			},
		})
	}

	ticketsQ := r.db.WithContext(ctx).Model(&schedulingmodels.QueueTicketModel{}).Where("org_id = ?", orgID)
	if branchID != nil && *branchID != uuid.Nil {
		ticketsQ = ticketsQ.Where("branch_id = ?", *branchID)
	}
	from := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	ticketsQ = ticketsQ.Where("requested_at >= ? AND requested_at < ?", from, to)
	var ticketRows []schedulingmodels.QueueTicketModel
	if err := ticketsQ.Order("requested_at ASC").Find(&ticketRows).Error; err != nil {
		return nil, err
	}
	for _, row := range ticketRows {
		ticket := toDomainQueueTicket(row)
		serviceID := ticket.ServiceID
		items = append(items, schedulingdomain.DayAgendaItem{
			Type:      "queue_ticket",
			ID:        ticket.ID,
			BranchID:  ticket.BranchID,
			ServiceID: serviceID,
			Status:    string(ticket.Status),
			Label:     ticket.DisplayCode + " " + strings.TrimSpace(ticket.CustomerName),
			Metadata: map[string]any{
				"queue_id": ticket.QueueID.String(),
				"number":   ticket.Number,
			},
		})
	}
	return items, nil
}

func (r *Repository) loadServiceResourceMap(ctx context.Context, serviceIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	out := make(map[uuid.UUID][]uuid.UUID, len(serviceIDs))
	if len(serviceIDs) == 0 {
		return out, nil
	}
	var rows []schedulingmodels.ServiceResourceModel
	if err := r.db.WithContext(ctx).
		Where("service_id IN ?", serviceIDs).
		Order("service_id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ServiceID] = append(out[row.ServiceID], row.ResourceID)
	}
	return out, nil
}

func toDomainBranch(row schedulingmodels.BranchModel) schedulingdomain.Branch {
	return schedulingdomain.Branch{
		ID:        row.ID,
		OrgID:     row.OrgID,
		Code:      row.Code,
		Name:      row.Name,
		Timezone:  row.Timezone,
		Address:   row.Address,
		Active:    row.Active,
		Metadata:  decodeJSON(row.Metadata),
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func toDomainService(row schedulingmodels.ServiceModel, resourceIDs []uuid.UUID) schedulingdomain.Service {
	return schedulingdomain.Service{
		ID:                     row.ID,
		OrgID:                  row.OrgID,
		CommercialServiceID:    row.CommercialServiceID,
		Code:                   row.Code,
		Name:                   row.Name,
		Description:            row.Description,
		FulfillmentMode:        schedulingdomain.FulfillmentMode(row.FulfillmentMode),
		DefaultDurationMinutes: row.DefaultDurationMinutes,
		BufferBeforeMinutes:    row.BufferBeforeMinutes,
		BufferAfterMinutes:     row.BufferAfterMinutes,
		SlotGranularityMinutes: row.SlotGranularityMinutes,
		MaxConcurrentBookings:  row.MaxConcurrentBookings,
		MinCancelNoticeMinutes: row.MinCancelNoticeMinutes,
		AllowWaitlist:          row.AllowWaitlist,
		Active:                 row.Active,
		ResourceIDs:            dedupeUUIDs(resourceIDs),
		Metadata:               decodeJSON(row.Metadata),
		CreatedAt:              row.CreatedAt,
		UpdatedAt:              row.UpdatedAt,
	}
}

func toDomainResource(row schedulingmodels.ResourceModel) schedulingdomain.Resource {
	return schedulingdomain.Resource{
		ID:        row.ID,
		OrgID:     row.OrgID,
		BranchID:  row.BranchID,
		Code:      row.Code,
		Name:      row.Name,
		Kind:      schedulingdomain.ResourceKind(row.Kind),
		Capacity:  row.Capacity,
		Timezone:  row.Timezone,
		Active:    row.Active,
		Metadata:  decodeJSON(row.Metadata),
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func toDomainAvailabilityRule(row schedulingmodels.AvailabilityRuleModel) schedulingdomain.AvailabilityRule {
	return schedulingdomain.AvailabilityRule{
		ID:                     row.ID,
		OrgID:                  row.OrgID,
		BranchID:               row.BranchID,
		ResourceID:             row.ResourceID,
		Kind:                   schedulingdomain.AvailabilityRuleKind(row.Kind),
		Weekday:                row.Weekday,
		StartTime:              row.StartTime,
		EndTime:                row.EndTime,
		SlotGranularityMinutes: row.SlotGranularityMinutes,
		ValidFrom:              row.ValidFrom,
		ValidUntil:             row.ValidUntil,
		Active:                 row.Active,
		Metadata:               decodeJSON(row.Metadata),
		CreatedAt:              row.CreatedAt,
		UpdatedAt:              row.UpdatedAt,
	}
}

func toDomainBlockedRange(row schedulingmodels.BlockedRangeModel) schedulingdomain.BlockedRange {
	return schedulingdomain.BlockedRange{
		ID:         row.ID,
		OrgID:      row.OrgID,
		BranchID:   row.BranchID,
		ResourceID: row.ResourceID,
		Kind:       schedulingdomain.BlockedRangeKind(row.Kind),
		Reason:     row.Reason,
		StartAt:    row.StartAt,
		EndAt:      row.EndAt,
		AllDay:     row.AllDay,
		CreatedBy:  row.CreatedBy,
		Metadata:   decodeJSON(row.Metadata),
		CreatedAt:  row.CreatedAt,
	}
}

func toDomainCalendarEvent(row schedulingmodels.CalendarEventModel) schedulingdomain.CalendarEvent {
	return schedulingdomain.CalendarEvent{
		ID:          row.ID,
		OrgID:       row.OrgID,
		BranchID:    row.BranchID,
		ResourceID:  row.ResourceID,
		Title:       row.Title,
		Description: row.Description,
		StartAt:     row.StartAt,
		EndAt:       row.EndAt,
		AllDay:      row.AllDay,
		Status:      schedulingdomain.CalendarEventStatus(row.Status),
		Visibility:  schedulingdomain.CalendarEventVisibility(row.Visibility),
		CreatedBy:   row.CreatedBy,
		Metadata:    decodeJSON(row.Metadata),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func toDomainBooking(row schedulingmodels.BookingModel) schedulingdomain.Booking {
	return schedulingdomain.Booking{
		ID:             row.ID,
		OrgID:          row.OrgID,
		BranchID:       row.BranchID,
		ServiceID:      row.ServiceID,
		ResourceID:     row.ResourceID,
		PartyID:        row.PartyID,
		Reference:      row.Reference,
		CustomerName:   row.CustomerName,
		CustomerPhone:  row.CustomerPhone,
		CustomerEmail:  row.CustomerEmail,
		Status:         schedulingdomain.BookingStatus(row.Status),
		Source:         schedulingdomain.BookingSource(row.Source),
		IdempotencyKey: derefString(row.IdempotencyKey),
		StartAt:        row.StartAt,
		EndAt:          row.EndAt,
		OccupiesFrom:   row.OccupiesFrom,
		OccupiesUntil:  row.OccupiesUntil,
		HoldExpiresAt:  row.HoldExpiresAt,
		Notes:          row.Notes,
		Metadata:       decodeJSON(row.Metadata),
		CreatedBy:      row.CreatedBy,
		ConfirmedAt:    row.ConfirmedAt,
		CancelledAt:    row.CancelledAt,
		ReminderSentAt: row.ReminderSentAt,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

func toDomainQueue(row schedulingmodels.QueueModel) schedulingdomain.Queue {
	return schedulingdomain.Queue{
		ID:               row.ID,
		OrgID:            row.OrgID,
		BranchID:         row.BranchID,
		ServiceID:        row.ServiceID,
		Code:             row.Code,
		Name:             row.Name,
		Status:           schedulingdomain.QueueStatus(row.Status),
		Strategy:         schedulingdomain.QueueStrategy(row.Strategy),
		TicketPrefix:     row.TicketPrefix,
		LastIssuedNumber: row.LastIssuedNumber,
		AvgServiceSecond: row.AvgServiceSecond,
		AllowRemoteJoin:  row.AllowRemoteJoin,
		Metadata:         decodeJSON(row.Metadata),
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func toDomainQueueTicket(row schedulingmodels.QueueTicketModel) schedulingdomain.QueueTicket {
	return schedulingdomain.QueueTicket{
		ID:                row.ID,
		OrgID:             row.OrgID,
		QueueID:           row.QueueID,
		BranchID:          row.BranchID,
		ServiceID:         row.ServiceID,
		PartyID:           row.PartyID,
		CustomerName:      row.CustomerName,
		CustomerPhone:     row.CustomerPhone,
		CustomerEmail:     row.CustomerEmail,
		Number:            row.Number,
		DisplayCode:       row.DisplayCode,
		Status:            schedulingdomain.QueueTicketStatus(row.Status),
		Priority:          row.Priority,
		Source:            schedulingdomain.QueueTicketSource(row.Source),
		IdempotencyKey:    row.IdempotencyKey,
		ServingResourceID: row.ServingResourceID,
		OperatorUserID:    row.OperatorUserID,
		RequestedAt:       row.RequestedAt,
		CalledAt:          row.CalledAt,
		StartedAt:         row.StartedAt,
		CompletedAt:       row.CompletedAt,
		CancelledAt:       row.CancelledAt,
		Notes:             row.Notes,
		Metadata:          decodeJSON(row.Metadata),
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func toDomainBookingActionToken(row schedulingmodels.BookingActionTokenModel) schedulingdomain.BookingActionToken {
	return schedulingdomain.BookingActionToken{
		ID:        row.ID,
		OrgID:     row.OrgID,
		BookingID: row.BookingID,
		Action:    schedulingdomain.BookingActionType(row.Action),
		TokenHash: row.TokenHash,
		ExpiresAt: row.ExpiresAt,
		UsedAt:    row.UsedAt,
		VoidedAt:  row.VoidedAt,
		Metadata:  decodeJSON(row.Metadata),
		CreatedAt: row.CreatedAt,
	}
}

func toDomainWaitlistEntry(row schedulingmodels.WaitlistEntryModel) schedulingdomain.WaitlistEntry {
	return schedulingdomain.WaitlistEntry{
		ID:               row.ID,
		OrgID:            row.OrgID,
		BranchID:         row.BranchID,
		ServiceID:        row.ServiceID,
		ResourceID:       row.ResourceID,
		PartyID:          row.PartyID,
		BookingID:        row.BookingID,
		CustomerName:     row.CustomerName,
		CustomerPhone:    row.CustomerPhone,
		CustomerEmail:    row.CustomerEmail,
		RequestedStartAt: row.RequestedStartAt,
		Status:           schedulingdomain.WaitlistStatus(row.Status),
		Source:           schedulingdomain.WaitlistSource(row.Source),
		IdempotencyKey:   row.IdempotencyKey,
		ExpiresAt:        row.ExpiresAt,
		NotifiedAt:       row.NotifiedAt,
		Notes:            row.Notes,
		Metadata:         decodeJSON(row.Metadata),
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func mustJSON(v any) []byte {
	if v == nil {
		return []byte(`{}`)
	}
	b, _ := json.Marshal(v)
	if len(b) == 0 {
		return []byte(`{}`)
	}
	return b
}

func decodeJSON(v []byte) map[string]any {
	if len(v) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(v, &out); err != nil {
		return nil
	}
	return out
}

// nullableTrimmed devuelve nil si el string queda vacío luego de TrimSpace.
// Se usa para columnas con índices únicos parciales `WHERE col IS NOT NULL`,
// donde `''` no se considera NULL y rompería la unicidad entre filas legítimas.
func nullableTrimmed(raw string) *string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func dedupeUUIDs(values []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(values))
	out := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		if value == uuid.Nil {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func buildDisplayCode(prefix string, n int64) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "T"
	}
	return fmt.Sprintf("%s-%03d", strings.ToUpper(prefix), n)
}

func expireOverdueHoldsTx(tx *gorm.DB, orgID, resourceID uuid.UUID, occupiesFrom, occupiesUntil time.Time) error {
	return tx.Model(&schedulingmodels.BookingModel{}).
		Where("org_id = ? AND resource_id = ? AND status = ? AND hold_expires_at IS NOT NULL AND hold_expires_at < ?", orgID, resourceID, string(schedulingdomain.BookingStatusHold), time.Now().UTC()).
		Where("occupies_from < ? AND occupies_until > ?", occupiesUntil.UTC(), occupiesFrom.UTC()).
		Updates(map[string]any{
			"status":     string(schedulingdomain.BookingStatusExpired),
			"updated_at": time.Now().UTC(),
		}).Error
}
