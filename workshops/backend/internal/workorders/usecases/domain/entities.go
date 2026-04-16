package domain

import (
	"time"

	"github.com/google/uuid"
)

// WorkOrder es la entidad unificada de órdenes de trabajo del vertical workshops.
// Soporta polimorfismo vía TargetType + TargetID (vehicle, bicycle, futuro: pet, asset, etc.).
// Cada vertical (auto_repair, bike_shop) consume el mismo dominio y enriquece comportamiento
// vía hooks (workorders.Hook).
type WorkOrder struct {
	ID       uuid.UUID
	OrgID    uuid.UUID
	BranchID *uuid.UUID
	Number   string

	// Polimorfismo: a qué activo apunta esta OT.
	TargetType  string    // 'vehicle' | 'bicycle' (extensible)
	TargetID    uuid.UUID // referencia opaca al asset
	TargetLabel string    // denormalizado: patente, "Trek Marlin 7", etc.

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

	CreatedBy  string
	ArchivedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time

	Items []WorkOrderItem
}

// WorkOrderItem es una línea de la OT (servicio o parte).
type WorkOrderItem struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
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
