package dto

type SalonServiceItem struct {
	ID              string  `json:"id"`
	OrgID           string  `json:"org_id"`
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Category        string  `json:"category"`
	DurationMinutes int     `json:"duration_minutes"`
	BasePrice       float64 `json:"base_price"`
	Currency        string  `json:"currency"`
	TaxRate         float64 `json:"tax_rate"`
	LinkedProductID *string `json:"linked_product_id,omitempty"`
	IsActive        bool    `json:"is_active"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type ListSalonServicesResponse struct {
	Items      []SalonServiceItem `json:"items"`
	Total      int64              `json:"total"`
	HasMore    bool               `json:"has_more"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

type CreateSalonServiceRequest struct {
	Code            string  `json:"code" binding:"required"`
	Name            string  `json:"name" binding:"required"`
	Description     string  `json:"description"`
	Category        string  `json:"category"`
	DurationMinutes int     `json:"duration_minutes"`
	BasePrice       float64 `json:"base_price"`
	Currency        string  `json:"currency"`
	TaxRate         float64 `json:"tax_rate"`
	LinkedProductID string  `json:"linked_product_id"`
	IsActive        *bool   `json:"is_active"`
}

type UpdateSalonServiceRequest struct {
	Code            *string  `json:"code"`
	Name            *string  `json:"name"`
	Description     *string  `json:"description"`
	Category        *string  `json:"category"`
	DurationMinutes *int     `json:"duration_minutes"`
	BasePrice       *float64 `json:"base_price"`
	Currency        *string  `json:"currency"`
	TaxRate         *float64 `json:"tax_rate"`
	LinkedProductID *string  `json:"linked_product_id"`
	IsActive        *bool    `json:"is_active"`
}
