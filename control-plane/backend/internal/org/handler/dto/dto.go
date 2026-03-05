package dto

type CreateOrgRequest struct {
	Name       string `json:"name" binding:"required"`
	Slug       string `json:"slug"`
	ExternalID string `json:"external_id"`
	Actor      string `json:"actor"`
}
