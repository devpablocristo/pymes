package domain

import (
	"time"

	"github.com/google/uuid"
)

// WorkOrder es la entidad unificada de órdenes de trabajo del vertical workshops.
// La OT apunta a un asset del cliente (asset_type + asset_id).
type WorkOrder struct {
	ID       uuid.UUID
	OrgID uuid.UUID
	BranchID *uuid.UUID
	Number   string

	AssetType  string    // 'vehicle' | 'bicycle' por compat de vertical; extensible.
	AssetID    uuid.UUID // referencia al customer_asset
	AssetLabel string    // denormalizado: patente, "Trek Marlin 7", etc.

	CustomerID   *uuid.UUID
	CustomerName string
	BookingID    *uuid.UUID
	QuoteID      *uuid.UUID
	SaleID       *uuid.UUID

	Status        string
	RequestedWork string
	Diagnosis     string
	Notes         string
	InternalNotes string

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

	// Metadata vertical-specific (segment, custom fields, etc.).
	Metadata map[string]any

	IsFavorite bool
	Tags       []string

	CreatedBy  string
	ArchivedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time

	Items []WorkOrderItem
}

// WorkOrderItem es una línea de la OT (servicio o parte).
type WorkOrderItem struct {
	ID          uuid.UUID
	OrgID    uuid.UUID
	WorkOrderID uuid.UUID
	ItemType    string // 'service' | 'part'
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
