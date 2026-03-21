package models

import (
	"time"

	"github.com/google/uuid"
)

type StaffMemberModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID       uuid.UUID `gorm:"type:uuid;index;not null"`
	DisplayName string    `gorm:"not null"`
	Role        string    `gorm:"not null;default:''"`
	Color       string    `gorm:"not null;default:'#6366f1'"`
	IsActive    bool      `gorm:"not null;default:true"`
	Notes       string    `gorm:"not null;default:''"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (StaffMemberModel) TableName() string { return "beauty.staff_members" }
