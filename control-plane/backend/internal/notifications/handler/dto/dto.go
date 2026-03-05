package dto

type UpdatePreferenceRequest struct {
	NotificationType string `json:"notification_type" binding:"required"`
	Channel          string `json:"channel" binding:"required"`
	Enabled          bool   `json:"enabled"`
}
