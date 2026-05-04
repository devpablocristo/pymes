package dto

type SaleItemPayload struct {
	ProductID   *string  `json:"product_id"`
	ServiceID   *string  `json:"service_id"`
	Description string   `json:"description"`
	Quantity    float64  `json:"quantity" binding:"required"`
	UnitPrice   float64  `json:"unit_price" binding:"required"`
	TaxRate     *float64 `json:"tax_rate"`
	SortOrder   int      `json:"sort_order"`
}

type CreateSaleRequest struct {
	BranchID      *string           `json:"branch_id"`
	CustomerID    *string           `json:"customer_id"`
	CustomerName  string            `json:"customer_name"`
	QuoteID       *string           `json:"quote_id"`
	PaymentMethod string            `json:"payment_method"`
	Items         []SaleItemPayload `json:"items" binding:"required"`
	Notes         string            `json:"notes"`
	Tags          []string          `json:"tags,omitempty"`
	Metadata      map[string]any    `json:"metadata,omitempty"`
}

// PatchSaleRequest actualización parcial desde el CRUD (etiquetas, metadata, notas, cobro, cliente).
type PatchSaleRequest struct {
	Tags          *[]string       `json:"tags"`
	Metadata      *map[string]any `json:"metadata"`
	Notes         *string         `json:"notes"`
	PaymentMethod *string         `json:"payment_method"`
	CustomerName  *string         `json:"customer_name"`
	BranchID      *string         `json:"branch_id"`
}

type SaleItemResponse struct {
	ID          string  `json:"id"`
	SaleID      string  `json:"sale_id"`
	ProductID   string  `json:"product_id,omitempty"`
	ServiceID   string  `json:"service_id,omitempty"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	CostPrice   float64 `json:"cost_price"`
	TaxRate     float64 `json:"tax_rate"`
	Subtotal    float64 `json:"subtotal"`
	SortOrder   int     `json:"sort_order"`
}

type SaleResponse struct {
	ID            string             `json:"id"`
	OrgID         string             `json:"org_id"`
	BranchID      string             `json:"branch_id,omitempty"`
	Number        string             `json:"number"`
	CustomerID    string             `json:"customer_id,omitempty"`
	CustomerName  string             `json:"customer_name"`
	QuoteID       string             `json:"quote_id,omitempty"`
	Status        string             `json:"status"`
	PaymentMethod string             `json:"payment_method"`
	Items         []SaleItemResponse `json:"items,omitempty"`
	Subtotal      float64            `json:"subtotal"`
	TaxTotal      float64            `json:"tax_total"`
	Total         float64            `json:"total"`
	Currency      string             `json:"currency"`
	Notes         string             `json:"notes"`
	CreatedBy     string             `json:"created_by"`
	CreatedAt     string             `json:"created_at"`
	Tags          []string           `json:"tags,omitempty"`
	Metadata      map[string]any     `json:"metadata,omitempty"`
}

type ListSalesResponse struct {
	Items      []SaleResponse `json:"items"`
	Total      int64          `json:"total"`
	HasMore    bool           `json:"has_more"`
	NextCursor string         `json:"next_cursor,omitempty"`
}
