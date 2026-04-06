package models

import (
	"time"

	"github.com/google/uuid"
)

type ServiceLinkModel struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID             uuid.UUID `gorm:"type:uuid;index;not null"`
	ProfileID         uuid.UUID `gorm:"type:uuid;not null"`
	ServiceID         uuid.UUID `gorm:"type:uuid;not null"`
	PublicDescription string
	DisplayOrder      int    `gorm:"not null;default:0"`
	IsFeatured        bool   `gorm:"not null;default:false"`
	Metadata          []byte `gorm:"type:jsonb"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (ServiceLinkModel) TableName() string { return "professionals.professional_service_links" }
