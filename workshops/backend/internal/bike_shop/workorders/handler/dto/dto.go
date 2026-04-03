package dto

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

type WorkOrderItem struct {
	ID               string              `json:"id"`
	OrgID            string              `json:"org_id"`
	Number           string              `json:"number"`
	BicycleID        string              `json:"bicycle_id"`
	BicycleLabel     string              `json:"bicycle_label"`
	CustomerID       *string             `json:"customer_id,omitempty"`
	CustomerName     string              `json:"customer_name"`
	BookingID    *string             `json:"booking_id,omitempty"`
	QuoteID          *string             `json:"quote_id,omitempty"`
	SaleID           *string             `json:"sale_id,omitempty"`
	Status           string              `json:"status"`
	RequestedWork    string              `json:"requested_work"`
	Diagnosis        string              `json:"diagnosis"`
	Notes            string              `json:"notes"`
	InternalNotes    string              `json:"internal_notes"`
	Currency         string              `json:"currency"`
	SubtotalServices float64             `json:"subtotal_services"`
	SubtotalParts    float64             `json:"subtotal_parts"`
	TaxTotal         float64             `json:"tax_total"`
	Total            float64             `json:"total"`
	OpenedAt         string              `json:"opened_at"`
	PromisedAt       *string             `json:"promised_at,omitempty"`
	ReadyAt          *string             `json:"ready_at,omitempty"`
	DeliveredAt      *string             `json:"delivered_at,omitempty"`
	CreatedBy        string              `json:"created_by"`
	CreatedAt        string              `json:"created_at"`
	UpdatedAt        string              `json:"updated_at"`
	Items            []WorkOrderLineItem `json:"items"`
}

type ListWorkOrdersResponse struct {
	Items      []WorkOrderItem `json:"items"`
	Total      int64           `json:"total"`
	HasMore    bool            `json:"has_more"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

type CreateWorkOrderRequest struct {
	Number        string               `json:"number"`
	BicycleID     string               `json:"bicycle_id" binding:"required"`
	BicycleLabel  string               `json:"bicycle_label"`
	CustomerID    string               `json:"customer_id"`
	CustomerName  string               `json:"customer_name"`
	BookingID string               `json:"booking_id"`
	Status        string               `json:"status"`
	RequestedWork string               `json:"requested_work"`
	Diagnosis     string               `json:"diagnosis"`
	Notes         string               `json:"notes"`
	InternalNotes string               `json:"internal_notes"`
	Currency      string               `json:"currency"`
	OpenedAt      string               `json:"opened_at"`
	PromisedAt    string               `json:"promised_at"`
	Items         []WorkOrderLineInput `json:"items" binding:"required"`
}

type UpdateWorkOrderRequest struct {
	BicycleID     *string               `json:"bicycle_id"`
	BicycleLabel  *string               `json:"bicycle_label"`
	CustomerID    *string               `json:"customer_id"`
	CustomerName  *string               `json:"customer_name"`
	BookingID *string               `json:"booking_id"`
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
