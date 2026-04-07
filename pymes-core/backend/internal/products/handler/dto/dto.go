package dto

type ProductItem struct {
	ID          string         `json:"id"`
	OrgID       string         `json:"org_id"`
	SKU         string         `json:"sku,omitempty"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Unit        string         `json:"unit"`
	Price       float64        `json:"price"`
	Currency    string         `json:"currency"`
	CostPrice   float64        `json:"cost_price"`
	TaxRate     *float64       `json:"tax_rate,omitempty"`
	ImageURL    string         `json:"image_url"`
	TrackStock  bool           `json:"track_stock"`
	IsActive    bool           `json:"is_active"`
	Tags        []string       `json:"tags"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
	DeletedAt   *string        `json:"deleted_at,omitempty"`
}

type ListProductsResponse struct {
	Items      []ProductItem `json:"items"`
	Total      int64         `json:"total"`
	HasMore    bool          `json:"has_more"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

type CreateProductRequest struct {
	SKU         string         `json:"sku"`
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description"`
	Unit        string         `json:"unit"`
	Price       float64        `json:"price"`
	Currency    string         `json:"currency"`
	CostPrice   float64        `json:"cost_price"`
	TaxRate     *float64       `json:"tax_rate"`
	ImageURL    string         `json:"image_url"`
	TrackStock  *bool          `json:"track_stock"`
	IsActive    *bool          `json:"is_active"`
	Tags        []string       `json:"tags"`
	Metadata    map[string]any `json:"metadata"`
}

type UpdateProductRequest struct {
	SKU         *string         `json:"sku"`
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	Unit        *string         `json:"unit"`
	Price       *float64        `json:"price"`
	Currency    *string         `json:"currency"`
	CostPrice   *float64        `json:"cost_price"`
	TaxRate     *float64        `json:"tax_rate"`
	ImageURL    *string         `json:"image_url"`
	TrackStock  *bool           `json:"track_stock"`
	IsActive    *bool           `json:"is_active"`
	Tags        *[]string       `json:"tags"`
	Metadata    *map[string]any `json:"metadata"`
}
