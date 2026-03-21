package dto

type StaffItem struct {
	ID          string `json:"id"`
	OrgID       string `json:"org_id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Color       string `json:"color"`
	IsActive    bool   `json:"is_active"`
	Notes       string `json:"notes"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type ListStaffResponse struct {
	Items      []StaffItem `json:"items"`
	Total      int64       `json:"total"`
	HasMore    bool        `json:"has_more"`
	NextCursor string      `json:"next_cursor,omitempty"`
}

type CreateStaffRequest struct {
	DisplayName string `json:"display_name" binding:"required"`
	Role        string `json:"role"`
	Color       string `json:"color"`
	IsActive    *bool  `json:"is_active"`
	Notes       string `json:"notes"`
}

type UpdateStaffRequest struct {
	DisplayName *string `json:"display_name"`
	Role        *string `json:"role"`
	Color       *string `json:"color"`
	IsActive    *bool   `json:"is_active"`
	Notes       *string `json:"notes"`
}
