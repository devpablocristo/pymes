package dto

type LineItemRequest struct {
	Description string  `json:"description"`
	Qty         float64 `json:"qty"`
	Unit        string  `json:"unit"`
	UnitPrice   float64 `json:"unit_price"`
	SortOrder   int     `json:"sort_order"`
}

type CreateInvoiceRequest struct {
	Number          string            `json:"number"`
	PartyID         *string           `json:"party_id"`
	CustomerName    string            `json:"customer_name"`
	IssuedDate      string            `json:"issued_date"`
	DueDate         string            `json:"due_date"`
	Status          string            `json:"status"`
	DiscountPercent float64           `json:"discount_percent"`
	TaxPercent      float64           `json:"tax_percent"`
	Notes           string            `json:"notes"`
	IsFavorite      *bool             `json:"is_favorite"`
	Tags            []string          `json:"tags"`
	Items           []LineItemRequest `json:"items"`
}

type UpdateInvoiceRequest struct {
	Status          *string   `json:"status"`
	DiscountPercent *float64  `json:"discount_percent"`
	TaxPercent      *float64  `json:"tax_percent"`
	Notes           *string   `json:"notes"`
	IsFavorite      *bool     `json:"is_favorite"`
	Tags            *[]string `json:"tags"`
	IssuedDate      *string   `json:"issued_date"`
	DueDate         *string   `json:"due_date"`
}

type LineItemResponse struct {
	ID          string  `json:"id"`
	InvoiceID   string  `json:"invoice_id"`
	Description string  `json:"description"`
	Qty         float64 `json:"qty"`
	Unit        string  `json:"unit"`
	UnitPrice   float64 `json:"unit_price"`
	LineTotal   float64 `json:"line_total"`
	SortOrder   int     `json:"sort_order"`
}

type InvoiceResponse struct {
	ID              string             `json:"id"`
	OrgID           string             `json:"org_id"`
	Number          string             `json:"number"`
	PartyID         string             `json:"party_id,omitempty"`
	CustomerName    string             `json:"customer_name"`
	IssuedDate      string             `json:"issued_date"`
	DueDate         string             `json:"due_date"`
	Status          string             `json:"status"`
	Subtotal        float64            `json:"subtotal"`
	DiscountPercent float64            `json:"discount_percent"`
	TaxPercent      float64            `json:"tax_percent"`
	Total           float64            `json:"total"`
	Notes           string             `json:"notes"`
	IsFavorite      bool               `json:"is_favorite"`
	Tags            []string           `json:"tags"`
	CreatedBy       string             `json:"created_by"`
	CreatedAt       string             `json:"created_at"`
	UpdatedAt       string             `json:"updated_at"`
	ArchivedAt      string             `json:"archived_at,omitempty"`
	Items           []LineItemResponse `json:"items"`
}

type ListInvoicesResponse struct {
	Items      []InvoiceResponse `json:"items"`
	Total      int64             `json:"total"`
	HasMore    bool              `json:"has_more"`
	NextCursor string            `json:"next_cursor,omitempty"`
}
