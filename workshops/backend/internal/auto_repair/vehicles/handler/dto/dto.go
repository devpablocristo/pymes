package dto

type VehicleItem struct {
	ID           string   `json:"id"`
	OrgID        string   `json:"org_id"`
	CustomerID   *string  `json:"customer_id,omitempty"`
	CustomerName string   `json:"customer_name"`
	LicensePlate string   `json:"license_plate"`
	VIN          string   `json:"vin"`
	Make         string   `json:"make"`
	Model        string   `json:"model"`
	Year         int      `json:"year"`
	Kilometers   int      `json:"kilometers"`
	Color        string   `json:"color"`
	Notes        string   `json:"notes"`
	IsFavorite   bool     `json:"is_favorite"`
	Tags         []string `json:"tags"`
	ArchivedAt   *string  `json:"archived_at,omitempty"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

type ListVehiclesResponse struct {
	Items      []VehicleItem `json:"items"`
	Total      int64         `json:"total"`
	HasMore    bool          `json:"has_more"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

type CreateVehicleRequest struct {
	CustomerID   string   `json:"customer_id"`
	CustomerName string   `json:"customer_name"`
	LicensePlate string   `json:"license_plate" binding:"required"`
	VIN          string   `json:"vin"`
	Make         string   `json:"make" binding:"required"`
	Model        string   `json:"model" binding:"required"`
	Year         int      `json:"year"`
	Kilometers   int      `json:"kilometers"`
	Color        string   `json:"color"`
	Notes        string   `json:"notes"`
	IsFavorite   *bool    `json:"is_favorite,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

type UpdateVehicleRequest struct {
	CustomerID   *string   `json:"customer_id"`
	CustomerName *string   `json:"customer_name"`
	LicensePlate *string   `json:"license_plate"`
	VIN          *string   `json:"vin"`
	Make         *string   `json:"make"`
	Model        *string   `json:"model"`
	Year         *int      `json:"year"`
	Kilometers   *int      `json:"kilometers"`
	Color        *string   `json:"color"`
	Notes        *string   `json:"notes"`
	IsFavorite   *bool     `json:"is_favorite,omitempty"`
	Tags         *[]string `json:"tags,omitempty"`
}
