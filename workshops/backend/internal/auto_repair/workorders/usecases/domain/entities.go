package domain

import (
	"time"

	"github.com/google/uuid"
)

type WorkOrder struct {
	ID               uuid.UUID
	OrgID            uuid.UUID
	Number           string
	VehicleID        uuid.UUID
	VehiclePlate     string
	CustomerID       *uuid.UUID
	CustomerName     string
	AppointmentID    *uuid.UUID
	QuoteID          *uuid.UUID
	SaleID           *uuid.UUID
	Status           string
	RequestedWork    string
	Diagnosis        string
	Notes            string
	InternalNotes    string
	Currency         string
	SubtotalServices float64
	SubtotalParts    float64
	TaxTotal         float64
	Total            float64
	OpenedAt              time.Time
	PromisedAt            *time.Time
	ReadyAt               *time.Time
	DeliveredAt           *time.Time
	ReadyPickupNotifiedAt *time.Time
	CreatedBy             string
	ArchivedAt            *time.Time
	CreatedAt             time.Time
	UpdatedAt        time.Time
	Items            []WorkOrderItem
}

type WorkOrderItem struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	WorkOrderID uuid.UUID
	ItemType    string
	ServiceID   *uuid.UUID
	ProductID   *uuid.UUID
	Description string
	Quantity    float64
	UnitPrice   float64
	TaxRate     float64
	SortOrder   int
	Metadata    map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
