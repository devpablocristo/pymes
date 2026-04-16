package dto

// WorkOrderLineInput es la entrada de una línea (servicio o parte) en create/update.
type WorkOrderLineInput struct {
	ItemType    string         `json:"item_type"`
	ServiceID   string         `json:"service_id"`
	ProductID   string         `json:"product_id"`
	Description string         `json:"description"`
	Quantity    float64        `json:"quantity"`
	UnitPrice   float64        `json:"unit_price"`
	TaxRate     float64        `json:"tax_rate"`
	Metadata    map[string]any `json:"metadata"`
}

// WorkOrderLineItem es la salida de una línea.
type WorkOrderLineItem struct {
	ID          string         `json:"id"`
	ItemType    string         `json:"item_type"`
	ServiceID   *string        `json:"service_id,omitempty"`
	ProductID   *string        `json:"product_id,omitempty"`
	Description string         `json:"description"`
	Quantity    float64        `json:"quantity"`
	UnitPrice   float64        `json:"unit_price"`
	TaxRate     float64        `json:"tax_rate"`
	SortOrder   int            `json:"sort_order"`
	Metadata    map[string]any `json:"metadata"`
}

// WorkOrderItem es el shape de salida unificado.
// Incluye los campos polimórficos (target_type/target_id/target_label) Y aliases por
// compatibilidad con clientes que esperan vehicle_id/vehicle_plate o bicycle_id/bicycle_label.
// Los aliases solo se llenan cuando target_type matchea.
type WorkOrderItem struct {
	ID       string `json:"id"`
	OrgID    string `json:"org_id"`
	BranchID string `json:"branch_id,omitempty"`
	Number   string `json:"number"`

	// Polimorfismo unificado.
	TargetType  string `json:"target_type"`
	TargetID    string `json:"target_id"`
	TargetLabel string `json:"target_label"`

	// Aliases por compat (solo se llenan si target_type matchea).
	VehicleID    string `json:"vehicle_id,omitempty"`
	VehiclePlate string `json:"vehicle_plate,omitempty"`
	BicycleID    string `json:"bicycle_id,omitempty"`
	BicycleLabel string `json:"bicycle_label,omitempty"`

	CustomerID   *string `json:"customer_id,omitempty"`
	CustomerName string  `json:"customer_name"`
	BookingID    *string `json:"booking_id,omitempty"`
	QuoteID      *string `json:"quote_id,omitempty"`
	SaleID       *string `json:"sale_id,omitempty"`

	Status        string `json:"status"`
	RequestedWork string `json:"requested_work"`
	Diagnosis     string `json:"diagnosis"`
	Notes         string `json:"notes"`
	InternalNotes string `json:"internal_notes"`

	Currency         string  `json:"currency"`
	SubtotalServices float64 `json:"subtotal_services"`
	SubtotalParts    float64 `json:"subtotal_parts"`
	TaxTotal         float64 `json:"tax_total"`
	Total            float64 `json:"total"`

	OpenedAt              string  `json:"opened_at"`
	PromisedAt            *string `json:"promised_at,omitempty"`
	ReadyAt               *string `json:"ready_at,omitempty"`
	DeliveredAt           *string `json:"delivered_at,omitempty"`
	ReadyPickupNotifiedAt *string `json:"ready_pickup_notified_at,omitempty"`

	Metadata map[string]any `json:"metadata"`

	CreatedBy  string  `json:"created_by"`
	ArchivedAt *string `json:"archived_at,omitempty"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`

	Items []WorkOrderLineItem `json:"items"`
}

type ListWorkOrdersResponse struct {
	Items      []WorkOrderItem `json:"items"`
	Total      int64           `json:"total"`
	HasMore    bool            `json:"has_more"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

// CreateWorkOrderRequest soporta tanto los nombres unificados como los aliases legacy.
// El handler resuelve cuál usar según target_type.
type CreateWorkOrderRequest struct {
	BranchID string `json:"branch_id"`
	Number   string `json:"number"`

	// Forma unificada (preferida).
	TargetType  string `json:"target_type"`
	TargetID    string `json:"target_id"`
	TargetLabel string `json:"target_label"`

	// Aliases legacy aceptados.
	VehicleID    string `json:"vehicle_id"`
	VehiclePlate string `json:"vehicle_plate"`
	BicycleID    string `json:"bicycle_id"`
	BicycleLabel string `json:"bicycle_label"`

	CustomerID    string               `json:"customer_id"`
	CustomerName  string               `json:"customer_name"`
	BookingID     string               `json:"booking_id"`
	Status        string               `json:"status"`
	RequestedWork string               `json:"requested_work"`
	Diagnosis     string               `json:"diagnosis"`
	Notes         string               `json:"notes"`
	InternalNotes string               `json:"internal_notes"`
	Currency      string               `json:"currency"`
	OpenedAt      string               `json:"opened_at"`
	PromisedAt    string               `json:"promised_at"`
	Metadata      map[string]any       `json:"metadata"`
	Items         []WorkOrderLineInput `json:"items" binding:"required"`
}

type UpdateWorkOrderRequest struct {
	BranchID *string `json:"branch_id"`
	// Unificados (preferidos).
	TargetID    *string `json:"target_id"`
	TargetLabel *string `json:"target_label"`

	// Aliases legacy aceptados.
	VehicleID    *string `json:"vehicle_id"`
	VehiclePlate *string `json:"vehicle_plate"`
	BicycleID    *string `json:"bicycle_id"`
	BicycleLabel *string `json:"bicycle_label"`

	CustomerID    *string               `json:"customer_id"`
	CustomerName  *string               `json:"customer_name"`
	BookingID     *string               `json:"booking_id"`
	Status        *string               `json:"status"`
	RequestedWork *string               `json:"requested_work"`
	Diagnosis     *string               `json:"diagnosis"`
	Notes         *string               `json:"notes"`
	InternalNotes *string               `json:"internal_notes"`
	Currency      *string               `json:"currency"`
	PromisedAt    *string               `json:"promised_at"`
	ReadyAt       *string               `json:"ready_at"`
	DeliveredAt   *string               `json:"delivered_at"`
	Items         *[]WorkOrderLineInput `json:"items"`
}
