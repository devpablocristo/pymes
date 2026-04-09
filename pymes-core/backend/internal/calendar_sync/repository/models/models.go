// Package models persiste conexiones de sync de calendario externo.
package models

import (
	"time"

	"github.com/google/uuid"
)

type CalendarSyncConnectionModel struct {
	ID                    uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID                 uuid.UUID  `gorm:"type:uuid;index;not null"`
	CreatedBy             string     `gorm:"column:created_by;not null"`
	Provider              string     `gorm:"not null"`
	ProviderAccountEmail  string     `gorm:"column:provider_account_email"`
	ProviderCalendarID    string     `gorm:"column:provider_calendar_id"`
	ProviderCalendarName  string     `gorm:"column:provider_calendar_name"`
	Scopes                string     `gorm:"column:scopes"`
	RefreshTokenEncrypted string     `gorm:"column:refresh_token_encrypted;not null"`
	AccessTokenEncrypted  string     `gorm:"column:access_token_encrypted"`
	AccessTokenExpiresAt  *time.Time `gorm:"column:access_token_expires_at"`
	SyncToken             string     `gorm:"column:sync_token"`
	LastSyncAt            *time.Time `gorm:"column:last_sync_at"`
	LastSyncError         string     `gorm:"column:last_sync_error"`
	RevokedAt             *time.Time `gorm:"column:revoked_at"`
	CreatedAt             time.Time  `gorm:"column:created_at"`
	UpdatedAt             time.Time  `gorm:"column:updated_at"`
}

func (CalendarSyncConnectionModel) TableName() string { return "calendar_sync_connections" }

type CalendarSyncOAuthStateModel struct {
	State     string    `gorm:"primaryKey"`
	OrgID     uuid.UUID `gorm:"type:uuid;not null"`
	CreatedBy string    `gorm:"column:created_by;not null"`
	Provider  string    `gorm:"not null"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (CalendarSyncOAuthStateModel) TableName() string { return "calendar_sync_oauth_states" }
