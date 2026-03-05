package models

import (
	"time"

	"github.com/google/uuid"
)

type OrgModel struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	ExternalID string    `gorm:"uniqueIndex"`
	Name       string
	Slug       string `gorm:"uniqueIndex"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (OrgModel) TableName() string { return "orgs" }
