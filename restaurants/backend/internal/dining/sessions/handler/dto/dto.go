package dto

type TableSessionItem struct {
	ID         string  `json:"id"`
	OrgID      string  `json:"org_id"`
	TableID    string  `json:"table_id"`
	TableCode  string  `json:"table_code,omitempty"`
	AreaName   string  `json:"area_name,omitempty"`
	GuestCount int     `json:"guest_count"`
	PartyLabel string  `json:"party_label"`
	Notes      string  `json:"notes"`
	OpenedAt   string  `json:"opened_at"`
	ClosedAt   *string `json:"closed_at,omitempty"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

type ListTableSessionsResponse struct {
	Items []TableSessionItem `json:"items"`
	Total int64              `json:"total"`
}

type OpenTableSessionRequest struct {
	TableID    string `json:"table_id" binding:"required"`
	GuestCount int    `json:"guest_count"`
	PartyLabel string `json:"party_label"`
	Notes      string `json:"notes"`
}
