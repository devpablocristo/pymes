package models

import (
	"time"

	"github.com/google/uuid"
)

type EndpointModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID     uuid.UUID `gorm:"type:uuid;index;not null"`
	URL       string    `gorm:"not null"`
	Secret    string    `gorm:"not null"`
	Events    []string  `gorm:"type:text[];not null"`
	IsActive  bool      `gorm:"not null;default:true"`
	CreatedBy string    `gorm:"default:''"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (EndpointModel) TableName() string { return "webhook_endpoints" }

type DeliveryModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	EndpointID   uuid.UUID `gorm:"type:uuid;index;not null"`
	EventType    string    `gorm:"not null"`
	Payload      []byte    `gorm:"type:jsonb;not null"`
	StatusCode   *int
	ResponseBody string `gorm:"not null;default:''"`
	Attempts     int    `gorm:"not null;default:0"`
	NextRetry    *time.Time
	DeliveredAt  *time.Time
	CreatedAt    time.Time `gorm:"not null"`
}

func (DeliveryModel) TableName() string { return "webhook_deliveries" }

type OutboxModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID `gorm:"type:uuid;index;not null"`
	EventType    string    `gorm:"not null"`
	Payload      []byte    `gorm:"type:jsonb;not null"`
	Status       string    `gorm:"not null;default:pending"`
	LastError    string    `gorm:"not null;default:''"`
	DispatchedAt *time.Time
	CreatedAt    time.Time `gorm:"not null"`
}

func (OutboxModel) TableName() string { return "webhook_outbox" }
