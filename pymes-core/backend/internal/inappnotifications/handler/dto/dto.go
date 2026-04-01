package dto

import (
	"encoding/json"
	"time"

	coredomain "github.com/devpablocristo/core/notifications/go/inbox/usecases/domain"
)

// InAppNotificationResponse DTO de salida.
type InAppNotificationResponse struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	Kind        string          `json:"kind"`
	EntityType  string          `json:"entity_type"`
	EntityID    string          `json:"entity_id"`
	ChatContext json.RawMessage `json:"chat_context"`
	ReadAt      *time.Time      `json:"read_at"`
	CreatedAt   time.Time       `json:"created_at"`
}

// PatchInAppNotificationRequest actualización parcial (marcar leída).
type PatchInAppNotificationRequest struct {
	Read *bool `json:"read"`
}

func MapNotification(n coredomain.Notification) InAppNotificationResponse {
	ctx := n.Metadata
	if len(ctx) == 0 {
		ctx = json.RawMessage(`{}`)
	}
	return InAppNotificationResponse{
		ID:          n.ID,
		Title:       n.Title,
		Body:        n.Body,
		Kind:        n.Kind,
		EntityType:  n.EntityType,
		EntityID:    n.EntityID,
		ChatContext: ctx,
		ReadAt:      n.ReadAt,
		CreatedAt:   n.CreatedAt,
	}
}
