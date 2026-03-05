package models

import (
	"time"

	"github.com/google/uuid"
)

type TenantSettingsModel struct {
	OrgID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	PlanCode   string    `gorm:"not null;default:starter"`
	HardLimits []byte    `gorm:"type:jsonb"`
	UpdatedBy  *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (TenantSettingsModel) TableName() string { return "tenant_settings" }

type AdminActivityEventModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID `gorm:"type:uuid;index;not null"`
	Actor        string
	Action       string
	ResourceType string
	ResourceID   string
	Payload      []byte `gorm:"type:jsonb"`
	CreatedAt    time.Time
}

func (AdminActivityEventModel) TableName() string { return "admin_activity_events" }
