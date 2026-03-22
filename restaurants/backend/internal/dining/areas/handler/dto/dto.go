package dto

type DiningAreaItem struct {
	ID        string `json:"id"`
	OrgID     string `json:"org_id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ListDiningAreasResponse struct {
	Items      []DiningAreaItem `json:"items"`
	Total      int64            `json:"total"`
	HasMore    bool             `json:"has_more"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

type CreateDiningAreaRequest struct {
	Name      string `json:"name" binding:"required"`
	SortOrder int    `json:"sort_order"`
}

type UpdateDiningAreaRequest struct {
	Name      *string `json:"name"`
	SortOrder *int    `json:"sort_order"`
}
