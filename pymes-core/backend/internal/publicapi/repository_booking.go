package publicapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/google/uuid"

	schedulingdomain "github.com/devpablocristo/modules/scheduling/go/domain"
	schedulingpublic "github.com/devpablocristo/modules/scheduling/go/publicapi"
)

type schedulingSelection struct {
	Branch   schedulingdomain.Branch
	Service  schedulingdomain.Service
	Resource *schedulingdomain.Resource
}

func (r *Repository) GetAvailability(ctx context.Context, orgID uuid.UUID, query AvailabilityQuery) ([]AvailabilitySlot, error) {
	if query.Duration < 0 || query.Duration > 720 {
		return nil, ErrInvalidInput
	}
	selection, ok, err := r.resolveSchedulingSelection(ctx, orgID, query.BranchID, query.ServiceID, query.ResourceID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("scheduling not configured for this tenant")
	}
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

func (r *Repository) Book(ctx context.Context, orgID uuid.UUID, payload map[string]any) (BookingPublic, error) {
	branchID, err := uuidPtrFromPayload(payload, "branch_id")
	if err != nil {
		return BookingPublic{}, ErrInvalidInput
	}
	serviceID, err := uuidPtrFromPayload(payload, "service_id")
	if err != nil {
		return BookingPublic{}, ErrInvalidInput
	}
	resourceID, err := uuidPtrFromPayload(payload, "resource_id")
	if err != nil {
		return BookingPublic{}, ErrInvalidInput
	}
	selection, ok, err := r.resolveSchedulingSelection(ctx, orgID, branchID, serviceID, resourceID)
	if err != nil {
		return BookingPublic{}, err
	}
	if !ok {
		return BookingPublic{}, fmt.Errorf("scheduling not configured for this tenant")
	}
	return r.bookScheduling(ctx, orgID, selection, payload)
}

func (r *Repository) ListByPhone(ctx context.Context, orgID uuid.UUID, phone string, limit int) ([]BookingPublic, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	phoneDigits := digitsOnly(phone)
	if phoneDigits == "" {
		return nil, ErrInvalidInput
	}
	return r.listSchedulingBookingsByPhone(ctx, orgID, phoneDigits, limit)
}

func (r *Repository) ConfirmBookingByToken(ctx context.Context, orgID uuid.UUID, token string) (BookingPublic, error) {
	if r.scheduling == nil {
		return BookingPublic{}, ErrInvalidInput
	}
	item, err := r.scheduling.ConfirmBookingByToken(ctx, token)
	if err != nil {
		return BookingPublic{}, mapSchedulingErr(err)
	}
	if item.OrgID != orgID {
		return BookingPublic{}, ErrInvalidInput
	}
	return bookingFromSchedulingBooking(item), nil
}

func (r *Repository) CancelBookingByToken(ctx context.Context, orgID uuid.UUID, token, reason string) (BookingPublic, error) {
	if r.scheduling == nil {
		return BookingPublic{}, ErrInvalidInput
	}
	item, err := r.scheduling.CancelBookingByToken(ctx, token, reason)
	if err != nil {
		return BookingPublic{}, mapSchedulingErr(err)
	}
	if item.OrgID != orgID {
		return BookingPublic{}, ErrInvalidInput
	}
	return bookingFromSchedulingBooking(item), nil
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

func (r *Repository) bookScheduling(ctx context.Context, orgID uuid.UUID, selection schedulingSelection, payload map[string]any) (BookingPublic, error) {
	startAt, err := timeValueFromPayload(payload, "start_at")
	if err != nil || startAt.IsZero() {
		return BookingPublic{}, ErrInvalidInput
	}
	customerName := firstStringFromPayload(payload, "party_name", "customer_name")
	customerPhone := firstStringFromPayload(payload, "party_phone", "customer_phone")
	customerEmail := firstStringFromPayload(payload, "customer_email")
	if strings.TrimSpace(customerName) == "" || strings.TrimSpace(customerPhone) == "" {
		return BookingPublic{}, ErrInvalidInput
	}
	partyID, err := uuidPtrFromPayload(payload, "party_id")
	if err != nil {
		return BookingPublic{}, ErrInvalidInput
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
		return BookingPublic{}, ErrInvalidInput
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
		CustomerEmail:  customerEmail,
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
		return BookingPublic{}, mapSchedulingErr(err)
	}
	title := strings.TrimSpace(customTitle)
	if title == "" {
		title = selection.Service.Name
	}
	actions := schedulingpublic.ActionLinks{}
	if r.scheduling != nil {
		if tokens, err := r.scheduling.CreateBookingActionTokens(ctx, orgID, booking.ID, 72*time.Hour); err == nil {
			actions = buildActionLinks(tokens)
		}
	}
	return BookingPublic{
		ID:            booking.ID,
		CustomerName:  booking.CustomerName,
		CustomerPhone: booking.CustomerPhone,
		CustomerEmail: booking.CustomerEmail,
		Title:         title,
		Status:        string(booking.Status),
		StartAt:       booking.StartAt.UTC(),
		EndAt:         booking.EndAt.UTC(),
		Duration:      int(booking.EndAt.Sub(booking.StartAt).Minutes()),
		ActionLinks:   actions,
	}, nil
}

func (r *Repository) listSchedulingBookingsByPhone(ctx context.Context, orgID uuid.UUID, phoneDigits string, limit int) ([]BookingPublic, error) {
	var rows []BookingPublic
	err := r.db.WithContext(ctx).
		Table("scheduling_bookings tb").
		Select(`
			tb.id,
			tb.customer_name as customer_name,
			tb.customer_phone as customer_phone,
			tb.customer_email as customer_email,
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
