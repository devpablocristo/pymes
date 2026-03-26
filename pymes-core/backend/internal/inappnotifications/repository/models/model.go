package models

import (
	"time"

	"github.com/google/uuid"
)

// InAppNotificationModel fila GORM de in_app_notifications.
type InAppNotificationModel struct {
	ID          uuid.UUID `gorm:"column:id;type:uuid;primaryKey"`
	OrgID       uuid.UUID `gorm:"column:org_id;type:uuid;not null"`
	UserID      uuid.UUID `gorm:"column:user_id;type:uuid;not null"`
	Title       string    `gorm:"column:title;not null"`
	Body        string    `gorm:"column:body;not null"`
	Kind        string    `gorm:"column:kind;not null"`
	EntityType  string    `gorm:"column:entity_type;not null"`
	EntityID    string    `gorm:"column:entity_id;not null"`
	ChatContext []byte    `gorm:"column:chat_context;type:jsonb;not null"`
	ReadAt      *time.Time `gorm:"column:read_at"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
}

func (InAppNotificationModel) TableName() string {
	return "in_app_notifications"
}
