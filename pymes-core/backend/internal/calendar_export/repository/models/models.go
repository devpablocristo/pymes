// Package models persiste los tokens de export de calendario.
package models

import (
	"time"

	"github.com/google/uuid"
)

type CalendarExportTokenModel struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID      uuid.UUID  `gorm:"type:uuid;index;not null"`
	CreatedBy  string     `gorm:"column:created_by;not null"`
	Name       string     `gorm:"not null"`
	TokenHash  string     `gorm:"column:token_hash;not null;uniqueIndex"`
	Scopes     string     `gorm:"not null"`
	LastUsedAt *time.Time `gorm:"column:last_used_at"`
	RevokedAt  *time.Time `gorm:"column:revoked_at"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
}

func (CalendarExportTokenModel) TableName() string { return "calendar_export_tokens" }
