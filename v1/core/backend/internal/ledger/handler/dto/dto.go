package dto

type CreateAccountRequest struct {
	Code       string  `json:"code" binding:"required"`
	Name       string  `json:"name" binding:"required"`
	Type       string  `json:"type" binding:"required"`
	ParentID   *string `json:"parent_id,omitempty"`
	IsPostable *bool   `json:"is_postable,omitempty"`
}

type UpdateAccountRequest struct {
	Name       string  `json:"name" binding:"required"`
	Type       string  `json:"type" binding:"required"`
	ParentID   *string `json:"parent_id,omitempty"`
	IsPostable *bool   `json:"is_postable,omitempty"`
}

type SetLinkRequest struct {
	AccountID string `json:"account_id" binding:"required"`
}

type EntryLineRequest struct {
	AccountID string  `json:"account_id" binding:"required"`
	Debit     float64 `json:"debit"`
	Credit    float64 `json:"credit"`
	PartyID   *string `json:"party_id,omitempty"`
	Memo      string  `json:"memo,omitempty"`
}

type PostEntryRequest struct {
	EntryDate   string             `json:"entry_date,omitempty"` // YYYY-MM-DD; default hoy
	Currency    string             `json:"currency,omitempty"`
	Description string             `json:"description,omitempty"`
	Lines       []EntryLineRequest `json:"lines" binding:"required"`
}
