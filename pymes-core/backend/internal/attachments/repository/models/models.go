package models

import (
	"time"

	"github.com/google/uuid"
)

type AttachmentModel struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID          uuid.UUID `gorm:"type:uuid;index;not null"`
	AttachableType string    `gorm:"not null"`
	AttachableID   uuid.UUID `gorm:"type:uuid;not null"`
	FileName       string    `gorm:"not null"`
	ContentType    string    `gorm:"not null;default:application/octet-stream"`
	SizeBytes      int64     `gorm:"not null;default:0"`
	StorageKey     string    `gorm:"not null"`
	UploadedBy     string    `gorm:"default:''"`
	CreatedAt      time.Time `gorm:"not null"`
}

func (AttachmentModel) TableName() string { return "attachments" }
