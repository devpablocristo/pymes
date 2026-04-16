package dto

type QuoteItemPayload struct {
	ProductID   *string  `json:"product_id"`
	ServiceID   *string  `json:"service_id"`
	Description string   `json:"description"`
	Quantity    float64  `json:"quantity" binding:"required"`
	UnitPrice   float64  `json:"unit_price" binding:"required"`
	TaxRate     *float64 `json:"tax_rate"`
	SortOrder   int      `json:"sort_order"`
}

type CreateQuoteRequest struct {
	BranchID     *string            `json:"branch_id"`
	CustomerID   *string            `json:"customer_id"`
	CustomerName string             `json:"customer_name"`
	Items        []QuoteItemPayload `json:"items" binding:"required"`
	Notes        string             `json:"notes"`
	ValidUntil   *string            `json:"valid_until"`
}

type UpdateQuoteRequest struct {
	CustomerID   *string             `json:"customer_id"`
	CustomerName *string             `json:"customer_name"`
	Items        *[]QuoteItemPayload `json:"items"`
	Notes        *string             `json:"notes"`
	ValidUntil   *string             `json:"valid_until"`
}

type ToSaleRequest struct {
	PaymentMethod string `json:"payment_method"`
	Notes         string `json:"notes"`
}

type QuoteItemResponse struct {
	ID          string  `json:"id"`
	QuoteID     string  `json:"quote_id"`
	ProductID   string  `json:"product_id,omitempty"`
	ServiceID   string  `json:"service_id,omitempty"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	TaxRate     float64 `json:"tax_rate"`
	Subtotal    float64 `json:"subtotal"`
	SortOrder   int     `json:"sort_order"`
}

type QuoteResponse struct {
	ID           string              `json:"id"`
	OrgID        string              `json:"org_id"`
	BranchID     string              `json:"branch_id,omitempty"`
	Number       string              `json:"number"`
	CustomerID   string              `json:"customer_id,omitempty"`
	CustomerName string              `json:"customer_name"`
	Status       string              `json:"status"`
	Items        []QuoteItemResponse `json:"items,omitempty"`
	Subtotal     float64             `json:"subtotal"`
	TaxTotal     float64             `json:"tax_total"`
	Total        float64             `json:"total"`
	Currency     string              `json:"currency"`
	Notes        string              `json:"notes"`
	ValidUntil   string              `json:"valid_until,omitempty"`
	CreatedBy    string              `json:"created_by"`
	CreatedAt    string              `json:"created_at"`
	UpdatedAt    string              `json:"updated_at"`
	ArchivedAt   string              `json:"archived_at,omitempty"`
}

type ListQuotesResponse struct {
	Items      []QuoteResponse `json:"items"`
	Total      int64           `json:"total"`
	HasMore    bool            `json:"has_more"`
	NextCursor string          `json:"next_cursor,omitempty"`
}
