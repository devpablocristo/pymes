package dto

type ProductItem struct {
	ID          string         `json:"id"`
	OrgID       string         `json:"org_id"`
	Type        string         `json:"type"`
	SKU         string         `json:"sku,omitempty"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Unit        string         `json:"unit"`
	Price       float64        `json:"price"`
	CostPrice   float64        `json:"cost_price"`
	TaxRate     *float64       `json:"tax_rate,omitempty"`
	TrackStock  bool           `json:"track_stock"`
	Tags        []string       `json:"tags"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

type ListProductsResponse struct {
	Items      []ProductItem `json:"items"`
	Total      int64         `json:"total"`
	HasMore    bool          `json:"has_more"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

type CreateProductRequest struct {
	Type        string         `json:"type"`
	SKU         string         `json:"sku"`
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description"`
	Unit        string         `json:"unit"`
	Price       float64        `json:"price"`
	CostPrice   float64        `json:"cost_price"`
	TaxRate     *float64       `json:"tax_rate"`
	TrackStock  *bool          `json:"track_stock"`
	Tags        []string       `json:"tags"`
	Metadata    map[string]any `json:"metadata"`
}

type UpdateProductRequest struct {
	Type        *string         `json:"type"`
	SKU         *string         `json:"sku"`
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	Unit        *string         `json:"unit"`
	Price       *float64        `json:"price"`
	CostPrice   *float64        `json:"cost_price"`
	TaxRate     *float64        `json:"tax_rate"`
	TrackStock  *bool           `json:"track_stock"`
	Tags        *[]string       `json:"tags"`
	Metadata    *map[string]any `json:"metadata"`
}
