package models

import (
	"time"

	"github.com/google/uuid"
)

type UserModel struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ExternalID string     `gorm:"uniqueIndex;not null"`
	Email      string     `gorm:"uniqueIndex;not null"`
	Name       string
	AvatarURL  string
	DeletedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (UserModel) TableName() string { return "users" }

type OrgMemberModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID     uuid.UUID `gorm:"type:uuid;index;not null"`
	UserID    uuid.UUID `gorm:"type:uuid;index;not null"`
	Role      string    `gorm:"not null;default:member"`
	CreatedAt time.Time
}

func (OrgMemberModel) TableName() string { return "org_members" }

type APIKeyModel struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID     uuid.UUID  `gorm:"type:uuid;index;not null"`
	Name      string
	KeyHash   string     `gorm:"uniqueIndex;not null"`
	KeyPrefix string
	CreatedBy string
	RotatedAt *time.Time
	CreatedAt time.Time
}

func (APIKeyModel) TableName() string { return "org_api_keys" }

type APIKeyScopeModel struct {
	ID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	KeyID uuid.UUID `gorm:"type:uuid;index;not null"`
	Scope string    `gorm:"not null"`
}

func (APIKeyScopeModel) TableName() string { return "org_api_key_scopes" }
