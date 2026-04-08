// Package publicapi implements public website and booking data access.
package publicapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"

	schedulingdomain "github.com/devpablocristo/modules/scheduling/go/domain"
	schedulingpublic "github.com/devpablocristo/modules/scheduling/go/publicapi"
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
	ListBookingsByPhone(ctx context.Context, orgID uuid.UUID, phone string, limit int) ([]schedulingdomain.Booking, error)
	CreateBookingActionTokens(ctx context.Context, orgID, bookingID uuid.UUID, ttl time.Duration) (map[schedulingdomain.BookingActionType]schedulingdomain.BookingActionToken, error)
	ConfirmBookingByToken(ctx context.Context, tokenRaw string) (schedulingdomain.Booking, error)
	CancelBookingByToken(ctx context.Context, tokenRaw, reason string) (schedulingdomain.Booking, error)
	ListQueues(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]schedulingdomain.Queue, error)
	IssueQueueTicket(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.CreateQueueTicketInput) (schedulingdomain.QueueTicket, error)
	GetQueueTicketPosition(ctx context.Context, orgID, queueID, ticketID uuid.UUID) (schedulingdomain.QueuePosition, error)
	JoinWaitlist(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.CreateWaitlistInput) (schedulingdomain.WaitlistEntry, error)
}

type Repository struct {
	db         *gorm.DB
	scheduling schedulingPort
}

func NewRepository(db *gorm.DB, scheduling schedulingPort) *Repository {
	return &Repository{db: db, scheduling: scheduling}
}

type BusinessInfo struct {
	OrgID             uuid.UUID `json:"org_id"`
	Name              string    `json:"name"`
	Slug              string    `json:"slug"`
	BusinessName      string    `json:"business_name"`
	BusinessAddress   string    `json:"business_address"`
	BusinessPhone     string    `json:"business_phone"`
	BusinessEmail     string    `json:"business_email"`
	SchedulingEnabled bool      `json:"scheduling_enabled"`
}

// PublicService es el shape legacy expuesto al adapter HTTP del módulo
// scheduling (publichttpgin). El endpoint público `/v1/public/:org_id/services`
// fue removido del handler local a favor de `catalog/services`, pero
// schedulingpublichttp.Handler aún consume este método del repo para su router.
type PublicService = schedulingpublic.Service
type AvailabilitySlot = schedulingpublic.AvailabilitySlot
type AvailabilityQuery = schedulingpublic.AvailabilityQuery
type BookingPublic = schedulingpublic.Booking

type orgResolveByIDRow struct {
	ID uuid.UUID `gorm:"column:id"`
}

type orgResolveBySlugRow struct {
	ID uuid.UUID
}

type businessInfoRow struct {
	OrgID             uuid.UUID
	Name              string
	Slug              string
	BusinessName      string
	BusinessAddress   string
	BusinessPhone     string
	BusinessEmail     string
	SchedulingEnabled bool
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
		var row orgResolveByIDRow
		err = r.db.WithContext(ctx).
			Table("orgs").
			Select("id").
			Where("id = ?", parsed).
			Take(&row).Error
		if err == nil {
			return row.ID, nil
		}
	}

	var row orgResolveBySlugRow
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
	var row businessInfoRow

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
			COALESCE(ts.scheduling_enabled, false) as scheduling_enabled
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
		OrgID:             row.OrgID,
		Name:              row.Name,
		Slug:              row.Slug,
		BusinessName:      businessName,
		BusinessAddress:   row.BusinessAddress,
		BusinessPhone:     row.BusinessPhone,
		BusinessEmail:     row.BusinessEmail,
		SchedulingEnabled: row.SchedulingEnabled,
	}, nil
}

// ListPublicServices mantiene el shape legacy para el adapter externo
// schedulingpublichttp. El handler local ya no la expone.
func (r *Repository) ListPublicServices(ctx context.Context, orgID uuid.UUID, limit int) ([]PublicService, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	if items, ok, err := r.listSchedulingPublicServices(ctx, orgID, limit); err != nil {
		return nil, err
	} else if ok {
		return items, nil
	}

	var rows []PublicService
	err := r.db.WithContext(ctx).
		Table("services").
		Select("id, name, 'service' as type, description, '' as unit, sale_price as price, currency").
		Where("org_id = ? AND deleted_at IS NULL AND is_active = true", orgID).
		Order("name ASC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
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
		// Catch-all services (used by the SMB owner from the internal calendar
		// to anote ad-hoc bookings) must never appear in the public catalog —
		// clients booking through PublicSchedulingFlow should only see real
		// catalog services with meaningful names and durations.
		if isCatchAllService(service.Metadata) {
			continue
		}
		unit := "booking"
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

// PublicServiceCatalogItem es el shape rico expuesto a verticales y al storefront
// para listar el catálogo de servicios desde public.services.
type PublicServiceCatalogItem struct {
	ID                     uuid.UUID      `json:"id"`
	Code                   string         `json:"code"`
	Name                   string         `json:"name"`
	Description            string         `json:"description"`
	CategoryCode           string         `json:"category_code"`
	SalePrice              float64        `json:"sale_price"`
	Currency               string         `json:"currency"`
	TaxRate                *float64       `json:"tax_rate,omitempty"`
	DefaultDurationMinutes *int           `json:"default_duration_minutes,omitempty"`
	Metadata               map[string]any `json:"metadata"`
}

type publicServiceCatalogRow struct {
	ID                     uuid.UUID
	Code                   string
	Name                   string
	Description            string
	CategoryCode           string
	SalePrice              float64
	Currency               string
	TaxRate                *float64
	DefaultDurationMinutes *int `gorm:"column:default_duration_minutes"`
	Metadata               []byte
}

// ListPublicServiceCatalog lee el catálogo rico desde public.services con filtros
// opcionales por metadata.vertical / metadata.segment y un search por nombre/código.
func (r *Repository) ListPublicServiceCatalog(ctx context.Context, orgID uuid.UUID, vertical, segment, search string, limit int) ([]PublicServiceCatalogItem, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})

	q := r.db.WithContext(ctx).
		Table("services").
		Select(`id, code, name, description, category_code, sale_price, currency,
			tax_rate, default_duration_minutes, metadata`).
		Where("org_id = ? AND deleted_at IS NULL AND is_active = true", orgID)

	if v := strings.TrimSpace(vertical); v != "" {
		q = q.Where("metadata->>'vertical' = ?", v)
	}
	if s := strings.TrimSpace(segment); s != "" {
		q = q.Where("metadata->>'segment' = ?", s)
	}
	if s := strings.TrimSpace(search); s != "" {
		like := "%" + s + "%"
		q = q.Where("(name ILIKE ? OR description ILIKE ? OR code ILIKE ?)", like, like, like)
	}

	var rows []publicServiceCatalogRow
	if err := q.Order("name ASC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]PublicServiceCatalogItem, 0, len(rows))
	for _, row := range rows {
		metadata := map[string]any{}
		if len(row.Metadata) > 0 {
			_ = json.Unmarshal(row.Metadata, &metadata)
		}
		out = append(out, PublicServiceCatalogItem{
			ID:                     row.ID,
			Code:                   row.Code,
			Name:                   row.Name,
			Description:            row.Description,
			CategoryCode:           row.CategoryCode,
			SalePrice:              row.SalePrice,
			Currency:               row.Currency,
			TaxRate:                row.TaxRate,
			DefaultDurationMinutes: row.DefaultDurationMinutes,
			Metadata:               metadata,
		})
	}
	return out, nil
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
		return nil, fmt.Errorf("scheduling not configured for this organization")
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
		return BookingPublic{}, fmt.Errorf("scheduling not configured for this organization")
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

func (r *Repository) ListPublicQueues(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]schedulingpublic.QueueSummary, error) {
	if r.scheduling == nil {
		return nil, nil
	}
	return r.scheduling.ListQueues(ctx, orgID, branchID)
}

func (r *Repository) CreatePublicQueueTicket(ctx context.Context, orgID, queueID uuid.UUID, payload map[string]any) (schedulingpublic.QueueTicket, schedulingpublic.QueuePosition, error) {
	if r.scheduling == nil {
		return schedulingpublic.QueueTicket{}, schedulingpublic.QueuePosition{}, ErrInvalidInput
	}
	partyID, err := uuidPtrFromPayload(payload, "party_id")
	if err != nil {
		return schedulingpublic.QueueTicket{}, schedulingpublic.QueuePosition{}, ErrInvalidInput
	}
	item, err := r.scheduling.IssueQueueTicket(ctx, orgID, "public-api", schedulingdomain.CreateQueueTicketInput{
		QueueID:        queueID,
		PartyID:        partyID,
		CustomerName:   firstStringFromPayload(payload, "customer_name", "party_name"),
		CustomerPhone:  firstStringFromPayload(payload, "customer_phone", "party_phone"),
		CustomerEmail:  firstStringFromPayload(payload, "customer_email"),
		Priority:       intValueFromPayload(payload, "priority"),
		Source:         schedulingdomain.QueueTicketSource(firstStringFromPayload(payload, "source")),
		IdempotencyKey: firstStringFromPayload(payload, "idempotency_key"),
		Notes:          firstStringFromPayload(payload, "notes"),
		Metadata:       ensureMap(payload["metadata"]),
	})
	if err != nil {
		return schedulingpublic.QueueTicket{}, schedulingpublic.QueuePosition{}, mapSchedulingErr(err)
	}
	position, err := r.scheduling.GetQueueTicketPosition(ctx, orgID, queueID, item.ID)
	if err != nil {
		return schedulingpublic.QueueTicket{}, schedulingpublic.QueuePosition{}, mapSchedulingErr(err)
	}
	return item, position, nil
}

func (r *Repository) GetPublicQueueTicketPosition(ctx context.Context, orgID, queueID, ticketID uuid.UUID) (schedulingpublic.QueuePosition, error) {
	if r.scheduling == nil {
		return schedulingpublic.QueuePosition{}, ErrInvalidInput
	}
	position, err := r.scheduling.GetQueueTicketPosition(ctx, orgID, queueID, ticketID)
	if err != nil {
		return schedulingpublic.QueuePosition{}, mapSchedulingErr(err)
	}
	return position, nil
}

func (r *Repository) JoinWaitlist(ctx context.Context, orgID uuid.UUID, payload map[string]any) (schedulingpublic.WaitlistEntry, error) {
	if r.scheduling == nil {
		return schedulingpublic.WaitlistEntry{}, ErrInvalidInput
	}
	branchID, err := uuidValueFromPayload(payload, "branch_id")
	if err != nil {
		return schedulingpublic.WaitlistEntry{}, ErrInvalidInput
	}
	serviceID, err := uuidValueFromPayload(payload, "service_id")
	if err != nil {
		return schedulingpublic.WaitlistEntry{}, ErrInvalidInput
	}
	resourceID, err := uuidPtrFromPayload(payload, "resource_id")
	if err != nil {
		return schedulingpublic.WaitlistEntry{}, ErrInvalidInput
	}
	partyID, err := uuidPtrFromPayload(payload, "party_id")
	if err != nil {
		return schedulingpublic.WaitlistEntry{}, ErrInvalidInput
	}
	requestedStartAt, err := timeValueFromPayload(payload, "requested_start_at")
	if err != nil {
		return schedulingpublic.WaitlistEntry{}, ErrInvalidInput
	}
	item, err := r.scheduling.JoinWaitlist(ctx, orgID, "public-api", schedulingdomain.CreateWaitlistInput{
		BranchID:         branchID,
		ServiceID:        serviceID,
		ResourceID:       resourceID,
		PartyID:          partyID,
		CustomerName:     firstStringFromPayload(payload, "customer_name", "party_name"),
		CustomerPhone:    firstStringFromPayload(payload, "customer_phone", "party_phone"),
		CustomerEmail:    firstStringFromPayload(payload, "customer_email"),
		RequestedStartAt: requestedStartAt.UTC(),
		Source:           schedulingdomain.WaitlistSource(firstStringFromPayload(payload, "source")),
		IdempotencyKey:   firstStringFromPayload(payload, "idempotency_key"),
		Notes:            firstStringFromPayload(payload, "notes"),
		Metadata:         ensureMap(payload["metadata"]),
	})
	if err != nil {
		return schedulingpublic.WaitlistEntry{}, mapSchedulingErr(err)
	}
	return item, nil
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

func bookingFromSchedulingBooking(item schedulingdomain.Booking) BookingPublic {
	return BookingPublic{
		ID:            item.ID,
		CustomerName:  item.CustomerName,
		CustomerPhone: item.CustomerPhone,
		CustomerEmail: item.CustomerEmail,
		Title:         item.Reference,
		Status:        string(item.Status),
		StartAt:       item.StartAt.UTC(),
		EndAt:         item.EndAt.UTC(),
		Duration:      int(item.EndAt.Sub(item.StartAt).Minutes()),
	}
}

func buildActionLinks(tokens map[schedulingdomain.BookingActionType]schedulingdomain.BookingActionToken) schedulingpublic.ActionLinks {
	out := schedulingpublic.ActionLinks{}
	if token, ok := tokens[schedulingdomain.BookingActionConfirm]; ok {
		out.ConfirmToken = token.Token
		out.ConfirmPath = "/scheduling/bookings/actions/confirm?token=" + token.Token
	}
	if token, ok := tokens[schedulingdomain.BookingActionCancel]; ok {
		out.CancelToken = token.Token
		out.CancelPath = "/scheduling/bookings/actions/cancel?token=" + token.Token
	}
	return out
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

func uuidValueFromPayload(payload map[string]any, key string) (uuid.UUID, error) {
	id, err := uuidPtrFromPayload(payload, key)
	if err != nil {
		return uuid.Nil, err
	}
	if id == nil {
		return uuid.Nil, ErrInvalidInput
	}
	return *id, nil
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

func digitsOnly(v string) string {
	var b strings.Builder
	for _, r := range v {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isCatchAllService reports whether a scheduling service is the owner-side
// catch-all (used to anote ad-hoc bookings from the internal calendar). Such
// services are flagged with metadata.catchall = true at seed time and must be
// hidden from the public catalog.
func isCatchAllService(metadata map[string]any) bool {
	if metadata == nil {
		return false
	}
	switch v := metadata["catchall"].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	default:
		return false
	}
}
