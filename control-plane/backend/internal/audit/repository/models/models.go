package models

import (
	"time"

	"github.com/google/uuid"
)

type AuditLogModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID `gorm:"type:uuid;index;not null"`
	Actor        string
	Action       string
	ResourceType string
	ResourceID   string
	Payload      []byte `gorm:"type:jsonb"`
	PrevHash     string
	Hash         string
	CreatedAt    time.Time
}

func (AuditLogModel) TableName() string { return "audit_log" }
