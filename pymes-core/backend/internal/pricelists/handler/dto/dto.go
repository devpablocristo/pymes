package dto

type PriceListItemPayload struct {
	ProductID *string `json:"product_id,omitempty"`
	ServiceID *string `json:"service_id,omitempty"`
	Price     float64 `json:"price" binding:"required"`
}

type CreatePriceListRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description,omitempty"`
	IsDefault   bool                   `json:"is_default,omitempty"`
	Markup      float64                `json:"markup,omitempty"`
	IsActive    *bool                  `json:"is_active,omitempty"`
	Items       []PriceListItemPayload `json:"items,omitempty"`
}
