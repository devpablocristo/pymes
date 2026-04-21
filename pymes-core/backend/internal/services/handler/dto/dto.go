package dto

type ServiceItem struct {
	ID                     string         `json:"id"`
	OrgID                  string         `json:"org_id"`
	Code                   string         `json:"code,omitempty"`
	Name                   string         `json:"name"`
	Description            string         `json:"description"`
	CategoryCode           string         `json:"category_code,omitempty"`
	SalePrice              float64        `json:"sale_price"`
	CostPrice              float64        `json:"cost_price"`
	TaxRate                *float64       `json:"tax_rate,omitempty"`
	Currency               string         `json:"currency"`
	DefaultDurationMinutes *int           `json:"default_duration_minutes,omitempty"`
	IsActive               bool           `json:"is_active"`
	IsFavorite             bool           `json:"is_favorite"`
	Tags                   []string       `json:"tags"`
	Metadata               map[string]any `json:"metadata"`
	CreatedAt              string         `json:"created_at"`
	UpdatedAt              string         `json:"updated_at"`
	DeletedAt              *string        `json:"deleted_at,omitempty"`
}

type ListServicesResponse struct {
	Items      []ServiceItem `json:"items"`
	Total      int64         `json:"total"`
	HasMore    bool          `json:"has_more"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

type CreateServiceRequest struct {
	Code                   string         `json:"code"`
	Name                   string         `json:"name" binding:"required"`
	Description            string         `json:"description"`
	CategoryCode           string         `json:"category_code"`
	SalePrice              float64        `json:"sale_price"`
	CostPrice              float64        `json:"cost_price"`
	TaxRate                *float64       `json:"tax_rate"`
	Currency               string         `json:"currency"`
	DefaultDurationMinutes *int           `json:"default_duration_minutes"`
	IsActive               *bool          `json:"is_active"`
	IsFavorite             *bool          `json:"is_favorite"`
	Tags                   []string       `json:"tags"`
	Metadata               map[string]any `json:"metadata"`
}

type UpdateServiceRequest struct {
	Code                   *string         `json:"code"`
	Name                   *string         `json:"name"`
	Description            *string         `json:"description"`
	CategoryCode           *string         `json:"category_code"`
	SalePrice              *float64        `json:"sale_price"`
	CostPrice              *float64        `json:"cost_price"`
	TaxRate                *float64        `json:"tax_rate"`
	Currency               *string         `json:"currency"`
	DefaultDurationMinutes *int            `json:"default_duration_minutes"`
	IsActive               *bool           `json:"is_active"`
	IsFavorite             *bool           `json:"is_favorite"`
	Tags                   *[]string       `json:"tags"`
	Metadata               *map[string]any `json:"metadata"`
}
