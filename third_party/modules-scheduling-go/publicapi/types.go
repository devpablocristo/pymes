package publicapi

import (
	"time"

	schedulingdomain "github.com/devpablocristo/modules/scheduling/go/domain"
	"github.com/google/uuid"
)

type Service struct {
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

type Booking struct {
	ID            uuid.UUID   `json:"id"`
	CustomerName  string      `json:"party_name"`
	CustomerPhone string      `json:"party_phone"`
	CustomerEmail string      `json:"customer_email,omitempty"`
	Title         string      `json:"title"`
	Status        string      `json:"status"`
	StartAt       time.Time   `json:"start_at"`
	EndAt         time.Time   `json:"end_at"`
	Duration      int         `json:"duration"`
	ActionLinks   ActionLinks `json:"actions,omitempty"`
}

type ActionLinks struct {
	ConfirmToken string `json:"confirm_token,omitempty"`
	CancelToken  string `json:"cancel_token,omitempty"`
	ConfirmPath  string `json:"confirm_path,omitempty"`
	CancelPath   string `json:"cancel_path,omitempty"`
}

type QueueSummary = schedulingdomain.Queue
type QueueTicket = schedulingdomain.QueueTicket
type QueuePosition = schedulingdomain.QueuePosition
type WaitlistEntry = schedulingdomain.WaitlistEntry
