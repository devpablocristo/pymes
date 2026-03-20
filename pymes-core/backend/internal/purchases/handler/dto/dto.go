package dto

type PurchaseItemPayload struct {
	ProductID   *string  `json:"product_id,omitempty"`
	Description string   `json:"description,omitempty"`
	Quantity    float64  `json:"quantity" binding:"required"`
	UnitCost    float64  `json:"unit_cost" binding:"required"`
	TaxRate     *float64 `json:"tax_rate,omitempty"`
}

type CreatePurchaseRequest struct {
	SupplierID    *string               `json:"supplier_id,omitempty"`
	SupplierName  string                `json:"supplier_name"`
	Status        string                `json:"status,omitempty"`
	PaymentStatus string                `json:"payment_status,omitempty"`
	Notes         string                `json:"notes,omitempty"`
	Items         []PurchaseItemPayload `json:"items" binding:"required"`
}
