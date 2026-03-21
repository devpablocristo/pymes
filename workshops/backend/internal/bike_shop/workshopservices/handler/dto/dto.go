package dto

type ServiceItem struct {
	ID              string  `json:"id"`
	OrgID           string  `json:"org_id"`
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Category        string  `json:"category"`
	EstimatedHours  float64 `json:"estimated_hours"`
	BasePrice       float64 `json:"base_price"`
	Currency        string  `json:"currency"`
	TaxRate         float64 `json:"tax_rate"`
	LinkedProductID *string `json:"linked_product_id,omitempty"`
	IsActive        bool    `json:"is_active"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type ListServicesResponse struct {
	Items      []ServiceItem `json:"items"`
	Total      int64         `json:"total"`
	HasMore    bool          `json:"has_more"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

type CreateServiceRequest struct {
	Code            string  `json:"code" binding:"required"`
	Name            string  `json:"name" binding:"required"`
	Description     string  `json:"description"`
	Category        string  `json:"category"`
	EstimatedHours  float64 `json:"estimated_hours"`
	BasePrice       float64 `json:"base_price"`
	Currency        string  `json:"currency"`
	TaxRate         float64 `json:"tax_rate"`
	LinkedProductID string  `json:"linked_product_id"`
	IsActive        *bool   `json:"is_active"`
}

type UpdateServiceRequest struct {
	Code            *string  `json:"code"`
	Name            *string  `json:"name"`
	Description     *string  `json:"description"`
	Category        *string  `json:"category"`
	EstimatedHours  *float64 `json:"estimated_hours"`
	BasePrice       *float64 `json:"base_price"`
	Currency        *string  `json:"currency"`
	TaxRate         *float64 `json:"tax_rate"`
	LinkedProductID *string  `json:"linked_product_id"`
	IsActive        *bool    `json:"is_active"`
}
