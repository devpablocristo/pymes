package dto

type ServiceLinkItem struct {
	ID                string         `json:"id"`
	OrgID             string         `json:"org_id"`
	ProfileID         string         `json:"profile_id"`
	ProductID         string         `json:"product_id"`
	PublicDescription string         `json:"public_description"`
	DisplayOrder      int            `json:"display_order"`
	IsFeatured        bool           `json:"is_featured"`
	Metadata          map[string]any `json:"metadata"`
	CreatedAt         string         `json:"created_at"`
	UpdatedAt         string         `json:"updated_at"`
}

type ListServiceLinksResponse struct {
	Items []ServiceLinkItem `json:"items"`
}

type ServiceLinkInput struct {
	ProductID         string         `json:"product_id" binding:"required"`
	PublicDescription string         `json:"public_description"`
	DisplayOrder      int            `json:"display_order"`
	IsFeatured        bool           `json:"is_featured"`
	Metadata          map[string]any `json:"metadata"`
}

type ReplaceServiceLinksRequest struct {
	Links []ServiceLinkInput `json:"links" binding:"required"`
}
