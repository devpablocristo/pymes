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
	Items         []PurchaseItemPayload `json:"items" binding:"required"`
}

type UpdatePurchaseStatusRequest struct {
	Status string `json:"status" binding:"required"`
}
