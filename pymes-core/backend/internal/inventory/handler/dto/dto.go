package dto

type StockLevelItem struct {
	ProductID   string  `json:"product_id"`
	OrgID       string  `json:"org_id"`
	BranchID    string  `json:"branch_id,omitempty"`
	ProductName string  `json:"product_name"`
	SKU         string  `json:"sku,omitempty"`
	Quantity    float64 `json:"quantity"`
	MinQuantity float64 `json:"min_quantity"`
	TrackStock  bool    `json:"track_stock"`
	IsLowStock  bool    `json:"is_low_stock"`
	UpdatedAt   string  `json:"updated_at"`
}

type ListStockResponse struct {
	Items      []StockLevelItem `json:"items"`
	Total      int64            `json:"total"`
	HasMore    bool             `json:"has_more"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

type AdjustStockRequest struct {
	Quantity    float64  `json:"quantity" binding:"required"`
	Notes       string   `json:"notes" binding:"required"`
	MinQuantity *float64 `json:"min_quantity"`
}

type StockMovementItem struct {
	ID          string  `json:"id"`
	OrgID       string  `json:"org_id"`
	BranchID    string  `json:"branch_id,omitempty"`
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Type        string  `json:"type"`
	Quantity    float64 `json:"quantity"`
	Reason      string  `json:"reason"`
	ReferenceID string  `json:"reference_id,omitempty"`
	Notes       string  `json:"notes"`
	CreatedBy   string  `json:"created_by"`
	CreatedAt   string  `json:"created_at"`
}

type ListMovementsResponse struct {
	Items      []StockMovementItem `json:"items"`
	Total      int64               `json:"total"`
	HasMore    bool                `json:"has_more"`
	NextCursor string              `json:"next_cursor,omitempty"`
}
