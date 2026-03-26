package models

import (
	"time"

	"github.com/google/uuid"
)

type AppointmentModel struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID         uuid.UUID  `gorm:"type:uuid;index;not null"`
	CustomerID    *uuid.UUID `gorm:"column:party_id;type:uuid"`
	CustomerName  string     `gorm:"column:party_name"`
	CustomerPhone string     `gorm:"column:party_phone"`
	Title         string
	Description   string
	Status        string
	StartAt       time.Time
	EndAt         time.Time
	Duration      int
	Location      string
	AssignedTo    string
	Color         string
	Notes         string
	Metadata      []byte `gorm:"type:jsonb"`
	CreatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ArchivedAt    *time.Time
}

func (AppointmentModel) TableName() string { return "appointments" }
