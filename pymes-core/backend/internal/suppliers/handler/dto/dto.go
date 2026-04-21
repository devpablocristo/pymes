package dto

type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	ZipCode string `json:"zip_code"`
	Country string `json:"country"`
}

type SupplierItem struct {
	ID          string         `json:"id"`
	OrgID       string         `json:"org_id"`
	Name        string         `json:"name"`
	TaxID       string         `json:"tax_id,omitempty"`
	Email       string         `json:"email,omitempty"`
	Phone       string         `json:"phone,omitempty"`
	Address     Address        `json:"address"`
	ContactName string         `json:"contact_name"`
	Notes       string         `json:"notes"`
	IsFavorite  bool           `json:"is_favorite"`
	Tags        []string       `json:"tags"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
	Archived    bool           `json:"archived"`
	DeletedAt   string         `json:"deleted_at,omitempty"`
}

type ListSuppliersResponse struct {
	Items      []SupplierItem `json:"items"`
	Total      int64          `json:"total"`
	HasMore    bool           `json:"has_more"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

type CreateSupplierRequest struct {
	Name        string         `json:"name" binding:"required"`
	TaxID       string         `json:"tax_id"`
	Email       string         `json:"email"`
	Phone       string         `json:"phone"`
	Address     Address        `json:"address"`
	ContactName string         `json:"contact_name"`
	Notes       string         `json:"notes"`
	IsFavorite  *bool          `json:"is_favorite"`
	Tags        []string       `json:"tags"`
	Metadata    map[string]any `json:"metadata"`
}

type UpdateSupplierRequest struct {
	Name        *string         `json:"name"`
	TaxID       *string         `json:"tax_id"`
	Email       *string         `json:"email"`
	Phone       *string         `json:"phone"`
	Address     *Address        `json:"address"`
	ContactName *string         `json:"contact_name"`
	Notes       *string         `json:"notes"`
	IsFavorite  *bool           `json:"is_favorite"`
	Tags        *[]string       `json:"tags"`
	Metadata    *map[string]any `json:"metadata"`
}
