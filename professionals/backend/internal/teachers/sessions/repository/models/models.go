package models

import (
	"time"

	"github.com/google/uuid"
)

type SessionModel struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID           uuid.UUID  `gorm:"type:uuid;index;not null"`
	AppointmentID   uuid.UUID  `gorm:"type:uuid;not null"`
	ProfileID       uuid.UUID  `gorm:"type:uuid;not null"`
	CustomerPartyID *uuid.UUID `gorm:"type:uuid"`
	ProductID       *uuid.UUID `gorm:"type:uuid"`
	Status          string     `gorm:"not null;default:scheduled"`
	StartedAt       *time.Time
	EndedAt         *time.Time
	Summary         string
	Metadata        []byte `gorm:"type:jsonb"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (SessionModel) TableName() string { return "professionals.sessions" }

type SessionNoteModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID     uuid.UUID `gorm:"type:uuid;not null"`
	SessionID uuid.UUID `gorm:"type:uuid;not null"`
	NoteType  string    `gorm:"not null;default:general"`
	Title     string
	Body      string `gorm:"not null"`
	CreatedBy string `gorm:"not null"`
	CreatedAt time.Time
}

func (SessionNoteModel) TableName() string { return "professionals.session_notes" }
