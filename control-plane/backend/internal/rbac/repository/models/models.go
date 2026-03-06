package models

import (
	"time"

	"github.com/google/uuid"
)

type RoleModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID       uuid.UUID `gorm:"type:uuid;index;not null"`
	Name        string    `gorm:"not null"`
	Description string
	IsSystem    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (RoleModel) TableName() string { return "roles" }

type RolePermissionModel struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	RoleID   uuid.UUID `gorm:"type:uuid;index;not null"`
	Resource string    `gorm:"not null"`
	Action   string    `gorm:"not null"`
}

func (RolePermissionModel) TableName() string { return "role_permissions" }

type UserRoleModel struct {
	UserID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID      uuid.UUID `gorm:"type:uuid;primaryKey;index;not null"`
	RoleID     uuid.UUID `gorm:"type:uuid;not null"`
	AssignedBy string
	AssignedAt time.Time
}

func (UserRoleModel) TableName() string { return "user_roles" }
