package dto

type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	ZipCode string `json:"zip_code"`
	Country string `json:"country"`
}

type CustomerItem struct {
	ID        string         `json:"id"`
	OrgID     string         `json:"org_id"`
	Type      string         `json:"type"`
	Name      string         `json:"name"`
	TaxID     string         `json:"tax_id,omitempty"`
	Email     string         `json:"email,omitempty"`
	Phone     string         `json:"phone,omitempty"`
	Address   Address        `json:"address"`
	Notes     string         `json:"notes"`
	IsFavorite bool          `json:"is_favorite"`
	Tags      []string       `json:"tags"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

type ListCustomersResponse struct {
	Items      []CustomerItem `json:"items"`
	Total      int64          `json:"total"`
	HasMore    bool           `json:"has_more"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

type CreateCustomerRequest struct {
	Type     string         `json:"type"`
	Name     string         `json:"name" binding:"required"`
	TaxID    string         `json:"tax_id"`
	Email    string         `json:"email"`
	Phone    string         `json:"phone"`
	Address  Address        `json:"address"`
	Notes    string         `json:"notes"`
	IsFavorite *bool        `json:"is_favorite"`
	Tags     []string       `json:"tags"`
	Metadata map[string]any `json:"metadata"`
}

type UpdateCustomerRequest struct {
	Type     *string         `json:"type"`
	Name     *string         `json:"name"`
	TaxID    *string         `json:"tax_id"`
	Email    *string         `json:"email"`
	Phone    *string         `json:"phone"`
	Address  *Address        `json:"address"`
	Notes    *string         `json:"notes"`
	IsFavorite *bool         `json:"is_favorite"`
	Tags     *[]string       `json:"tags"`
	Metadata *map[string]any `json:"metadata"`
}

type SaleHistoryItem struct {
	ID            string  `json:"id"`
	Number        string  `json:"number"`
	Status        string  `json:"status"`
	PaymentMethod string  `json:"payment_method"`
	Total         float64 `json:"total"`
	Currency      string  `json:"currency"`
	CreatedAt     string  `json:"created_at"`
}

type ListSalesHistoryResponse struct {
	Items []SaleHistoryItem `json:"items"`
}

type ImportCustomersResponse struct {
	Imported int `json:"imported"`
}
