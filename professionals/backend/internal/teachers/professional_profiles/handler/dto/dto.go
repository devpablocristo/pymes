package dto

type ProfileItem struct {
	ID                string         `json:"id"`
	OrgID             string         `json:"org_id"`
	PartyID           string         `json:"party_id"`
	PublicSlug        string         `json:"public_slug"`
	Bio               string         `json:"bio"`
	Headline          string         `json:"headline"`
	IsPublic          bool           `json:"is_public"`
	IsBookable        bool           `json:"is_bookable"`
	AcceptsNewClients bool           `json:"accepts_new_clients"`
	IsFavorite        bool           `json:"is_favorite"`
	Tags              []string       `json:"tags"`
	Metadata          map[string]any `json:"metadata"`
	Specialties       []SpecialtyRef `json:"specialties,omitempty"`
	CreatedAt         string         `json:"created_at"`
	UpdatedAt         string         `json:"updated_at"`
}

type SpecialtyRef struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type ListProfilesResponse struct {
	Items      []ProfileItem `json:"items"`
	Total      int64         `json:"total"`
	HasMore    bool          `json:"has_more"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

type CreateProfileRequest struct {
	PartyID           string         `json:"party_id" binding:"required"`
	PublicSlug        string         `json:"public_slug"`
	Bio               string         `json:"bio"`
	Headline          string         `json:"headline"`
	IsPublic          *bool          `json:"is_public"`
	IsBookable        *bool          `json:"is_bookable"`
	AcceptsNewClients *bool          `json:"accepts_new_clients"`
	IsFavorite        *bool          `json:"is_favorite,omitempty"`
	Tags              []string       `json:"tags,omitempty"`
	Metadata          map[string]any `json:"metadata"`
}

type UpdateProfileRequest struct {
	PublicSlug        *string         `json:"public_slug"`
	Bio               *string         `json:"bio"`
	Headline          *string         `json:"headline"`
	IsPublic          *bool           `json:"is_public"`
	IsBookable        *bool           `json:"is_bookable"`
	AcceptsNewClients *bool           `json:"accepts_new_clients"`
	IsFavorite        *bool           `json:"is_favorite,omitempty"`
	Tags              *[]string       `json:"tags,omitempty"`
	Metadata          *map[string]any `json:"metadata"`
}
