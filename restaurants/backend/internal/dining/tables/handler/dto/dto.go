package dto

type DiningTableItem struct {
	ID        string `json:"id"`
	OrgID     string `json:"org_id"`
	AreaID    string `json:"area_id"`
	Code      string `json:"code"`
	Label     string `json:"label"`
	Capacity  int    `json:"capacity"`
	Status    string `json:"status"`
	Notes     string `json:"notes"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ListDiningTablesResponse struct {
	Items      []DiningTableItem `json:"items"`
	Total      int64             `json:"total"`
	HasMore    bool              `json:"has_more"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

type CreateDiningTableRequest struct {
	AreaID   string `json:"area_id" binding:"required"`
	Code     string `json:"code" binding:"required"`
	Label    string `json:"label"`
	Capacity int    `json:"capacity"`
	Status   string `json:"status"`
	Notes    string `json:"notes"`
}

type UpdateDiningTableRequest struct {
	AreaID   *string `json:"area_id"`
	Code     *string `json:"code"`
	Label    *string `json:"label"`
	Capacity *int    `json:"capacity"`
	Status   *string `json:"status"`
	Notes    *string `json:"notes"`
}
