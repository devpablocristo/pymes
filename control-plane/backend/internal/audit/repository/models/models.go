package models

import (
	"time"

	"github.com/google/uuid"
)

type AuditLogModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID `gorm:"type:uuid;index;not null"`
	Actor        string
	ActorType    string     `gorm:"not null;default:user"`
	ActorID      *uuid.UUID `gorm:"type:uuid"`
	ActorLabel   string     `gorm:"not null;default:''"`
	Action       string
	ResourceType string
	ResourceID   string
	Payload      []byte `gorm:"type:jsonb"`
	PrevHash     string
	Hash         string
	CreatedAt    time.Time
}

func (AuditLogModel) TableName() string { return "audit_log" }
