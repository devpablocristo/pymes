package dto

type CreateAPIKeyRequest struct {
	Name   string   `json:"name" binding:"required"`
	Scopes []string `json:"scopes"`
}

type RotateAPIKeyResponse struct {
	RawKey string `json:"raw_key"`
}

type NotificationPreferenceRequest struct {
	NotificationType string `json:"notification_type" binding:"required"`
	Channel          string `json:"channel" binding:"required"`
	Enabled          bool   `json:"enabled"`
}
