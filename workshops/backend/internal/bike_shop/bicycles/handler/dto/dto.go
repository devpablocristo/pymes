package dto

type BicycleItem struct {
	ID              string   `json:"id"`
	OrgID           string   `json:"org_id"`
	CustomerID      *string  `json:"customer_id,omitempty"`
	CustomerName    string   `json:"customer_name"`
	FrameNumber     string   `json:"frame_number"`
	Brand           string   `json:"brand"`
	Model           string   `json:"model"`
	BikeType        string   `json:"bike_type"`
	Size            string   `json:"size"`
	WheelSizeInches int      `json:"wheel_size_inches"`
	Color           string   `json:"color"`
	EbikeNotes      string   `json:"ebike_notes"`
	Notes           string   `json:"notes"`
	IsFavorite      bool     `json:"is_favorite"`
	Tags            []string `json:"tags"`
	ArchivedAt      *string  `json:"archived_at,omitempty"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

type ListBicyclesResponse struct {
	Items      []BicycleItem `json:"items"`
	Total      int64         `json:"total"`
	HasMore    bool          `json:"has_more"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

type CreateBicycleRequest struct {
	CustomerID      string   `json:"customer_id"`
	CustomerName    string   `json:"customer_name"`
	FrameNumber     string   `json:"frame_number" binding:"required"`
	Brand           string   `json:"brand" binding:"required"`
	Model           string   `json:"model" binding:"required"`
	BikeType        string   `json:"bike_type"`
	Size            string   `json:"size"`
	WheelSizeInches int      `json:"wheel_size_inches"`
	Color           string   `json:"color"`
	EbikeNotes      string   `json:"ebike_notes"`
	Notes           string   `json:"notes"`
	IsFavorite      *bool    `json:"is_favorite,omitempty"`
	Tags            []string `json:"tags,omitempty"`
}

type UpdateBicycleRequest struct {
	CustomerID      *string   `json:"customer_id"`
	CustomerName    *string   `json:"customer_name"`
	FrameNumber     *string   `json:"frame_number"`
	Brand           *string   `json:"brand"`
	Model           *string   `json:"model"`
	BikeType        *string   `json:"bike_type"`
	Size            *string   `json:"size"`
	WheelSizeInches *int      `json:"wheel_size_inches"`
	Color           *string   `json:"color"`
	EbikeNotes      *string   `json:"ebike_notes"`
	Notes           *string   `json:"notes"`
	IsFavorite      *bool     `json:"is_favorite,omitempty"`
	Tags            *[]string `json:"tags,omitempty"`
}
