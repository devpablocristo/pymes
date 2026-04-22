package dto

type SpecialtyItem struct {
	ID          string   `json:"id"`
	OrgID       string   `json:"org_id"`
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IsActive    bool     `json:"is_active"`
	IsFavorite  bool     `json:"is_favorite"`
	Tags        []string `json:"tags"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type ListSpecialtiesResponse struct {
	Items      []SpecialtyItem `json:"items"`
	Total      int64           `json:"total"`
	HasMore    bool            `json:"has_more"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

type CreateSpecialtyRequest struct {
	Code        string   `json:"code" binding:"required"`
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	IsActive    *bool    `json:"is_active"`
	IsFavorite  *bool    `json:"is_favorite,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type UpdateSpecialtyRequest struct {
	Code        *string   `json:"code"`
	Name        *string   `json:"name"`
	Description *string   `json:"description"`
	IsActive    *bool     `json:"is_active"`
	IsFavorite  *bool     `json:"is_favorite,omitempty"`
	Tags        *[]string `json:"tags,omitempty"`
}

type AssignProfessionalsRequest struct {
	ProfileIDs []string `json:"profile_ids" binding:"required"`
}
