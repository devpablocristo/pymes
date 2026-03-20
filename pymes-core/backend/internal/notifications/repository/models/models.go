package models

import (
	"time"

	"github.com/google/uuid"
)

type NotificationPreferenceModel struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID           uuid.UUID `gorm:"type:uuid;index;not null"`
	NotificationType string    `gorm:"not null"`
	Channel          string    `gorm:"not null"`
	Enabled          bool      `gorm:"not null;default:true"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (NotificationPreferenceModel) TableName() string { return "notification_preferences" }

type NotificationLogModel struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID             uuid.UUID `gorm:"type:uuid;index;not null"`
	UserID            uuid.UUID `gorm:"type:uuid;index;not null"`
	NotificationType  string
	Channel           string
	Status            string
	ProviderMessageID string
	DedupKey          string `gorm:"uniqueIndex;not null"`
	CreatedAt         time.Time
}

func (NotificationLogModel) TableName() string { return "notification_log" }
