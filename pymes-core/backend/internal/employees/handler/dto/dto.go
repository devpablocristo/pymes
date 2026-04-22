package dto

type CreateEmployeeRequest struct {
	FirstName  string   `json:"first_name"`
	LastName   string   `json:"last_name"`
	Email      string   `json:"email"`
	Phone      string   `json:"phone"`
	Position   string   `json:"position"`
	Status     string   `json:"status"`
	HireDate   string   `json:"hire_date"`
	EndDate    string   `json:"end_date"`
	Notes      string   `json:"notes"`
	IsFavorite *bool    `json:"is_favorite"`
	Tags       []string `json:"tags"`
}

type UpdateEmployeeRequest struct {
	FirstName  *string   `json:"first_name"`
	LastName   *string   `json:"last_name"`
	Email      *string   `json:"email"`
	Phone      *string   `json:"phone"`
	Position   *string   `json:"position"`
	Status     *string   `json:"status"`
	HireDate   *string   `json:"hire_date"`
	EndDate    *string   `json:"end_date"`
	Notes      *string   `json:"notes"`
	IsFavorite *bool     `json:"is_favorite"`
	Tags       *[]string `json:"tags"`
}

type EmployeeResponse struct {
	ID         string   `json:"id"`
	OrgID      string   `json:"org_id"`
	FirstName  string   `json:"first_name"`
	LastName   string   `json:"last_name"`
	Email      string   `json:"email"`
	Phone      string   `json:"phone"`
	Position   string   `json:"position"`
	Status     string   `json:"status"`
	HireDate   string   `json:"hire_date,omitempty"`
	EndDate    string   `json:"end_date,omitempty"`
	UserID     string   `json:"user_id,omitempty"`
	Notes      string   `json:"notes"`
	IsFavorite bool     `json:"is_favorite"`
	Tags       []string `json:"tags"`
	CreatedBy  string   `json:"created_by"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
	ArchivedAt string   `json:"archived_at,omitempty"`
}

type ListEmployeesResponse struct {
	Items      []EmployeeResponse `json:"items"`
	Total      int64              `json:"total"`
	HasMore    bool               `json:"has_more"`
	NextCursor string             `json:"next_cursor,omitempty"`
}
