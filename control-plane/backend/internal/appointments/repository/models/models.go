package models

import (
	"time"

	"github.com/google/uuid"
)

type AppointmentModel struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID         uuid.UUID `gorm:"type:uuid;index;not null"`
	CustomerID    *uuid.UUID
	CustomerName  string
	CustomerPhone string
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
}

func (AppointmentModel) TableName() string { return "appointments" }
