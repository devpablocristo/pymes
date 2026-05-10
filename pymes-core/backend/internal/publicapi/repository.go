// Package publicapi implements public website and booking data access.
package publicapi

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	schedulingdomain "github.com/devpablocristo/modules/scheduling/go/domain"
	schedulingpublic "github.com/devpablocristo/modules/scheduling/go/publicapi"
)

var (
	ErrTenantNotFound  = errors.New("tenant not found")
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
	OrgID          uuid.UUID `json:"org_id"`
	Name              string    `json:"name"`
	Slug              string    `json:"slug"`
	BusinessName      string    `json:"business_name"`
	BusinessAddress   string    `json:"business_address"`
	BusinessPhone     string    `json:"business_phone"`
	BusinessEmail     string    `json:"business_email"`
	SchedulingEnabled bool      `json:"scheduling_enabled"`
}

// PublicService is the compact service shape consumed by the scheduling public HTTP adapter.
type PublicService = schedulingpublic.Service
type AvailabilitySlot = schedulingpublic.AvailabilitySlot
type AvailabilityQuery = schedulingpublic.AvailabilityQuery
type BookingPublic = schedulingpublic.Booking
