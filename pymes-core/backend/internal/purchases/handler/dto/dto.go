package dto

type PurchaseItemPayload struct {
	ProductID   *string  `json:"product_id,omitempty"`
	ServiceID   *string  `json:"service_id,omitempty"`
	Description string   `json:"description,omitempty"`
	Quantity    float64  `json:"quantity" binding:"required"`
	UnitCost    float64  `json:"unit_cost" binding:"required"`
	TaxRate     *float64 `json:"tax_rate,omitempty"`
}

type CreatePurchaseRequest struct {
	BranchID      *string               `json:"branch_id,omitempty"`
	SupplierID    *string               `json:"supplier_id,omitempty"`
	SupplierName  string                `json:"supplier_name"`
	Status        string                `json:"status,omitempty"`
	PaymentStatus string                `json:"payment_status,omitempty"`
	Notes         string                `json:"notes,omitempty"`
	Tags          []string              `json:"tags,omitempty"`
	Metadata      map[string]any        `json:"metadata,omitempty"`
	Items         []PurchaseItemPayload `json:"items" binding:"required"`
}

// PatchPurchaseRequest actualización parcial de campos editables fuera del flujo PUT borrador.
type PatchPurchaseRequest struct {
	Tags          *[]string       `json:"tags"`
	Metadata      *map[string]any `json:"metadata"`
	Notes         *string         `json:"notes"`
	PaymentStatus *string         `json:"payment_status"`
	SupplierName  *string         `json:"supplier_name"`
}

type UpdatePurchaseStatusRequest struct {
	Status string `json:"status" binding:"required"`
}
