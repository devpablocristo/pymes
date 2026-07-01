package models

import (
	"time"

	"github.com/google/uuid"
)

type AccountModel struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID      uuid.UUID  `gorm:"type:uuid;index;not null"`
	Code       string     `gorm:"not null"`
	Name       string     `gorm:"not null"`
	Type       string     `gorm:"type:char(1);not null"`
	ParentID   *uuid.UUID `gorm:"type:uuid"`
	IsPostable bool       `gorm:"not null;default:true"`
	ArchivedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (AccountModel) TableName() string { return "ledger_accounts" }

type AccountLinkModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID     uuid.UUID `gorm:"type:uuid;index;not null"`
	Role      string    `gorm:"not null"`
	AccountID uuid.UUID `gorm:"type:uuid;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (AccountLinkModel) TableName() string { return "ledger_account_links" }

type SequenceModel struct {
	OrgID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	NextEntryNumber int64     `gorm:"not null;default:1"`
}

func (SequenceModel) TableName() string { return "ledger_sequences" }

type JournalEntryModel struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID             uuid.UUID  `gorm:"type:uuid;index;not null"`
	EntryNumber       string     `gorm:"not null"`
	EntryDate         time.Time  `gorm:"type:date;not null"`
	Currency          string     `gorm:"not null;default:ARS"`
	ExchangeRate      float64    `gorm:"not null;default:1"`
	SourceType        string     `gorm:"not null;default:manual"`
	SourceID          *uuid.UUID `gorm:"type:uuid"`
	SourceEvent       string     `gorm:"not null;default:manual"`
	Description       string     `gorm:"not null;default:''"`
	Status            string     `gorm:"not null;default:posted"`
	ReversedByEntryID *uuid.UUID `gorm:"type:uuid"`
	CreatedBy         string     `gorm:"not null;default:''"`
	CreatedAt         time.Time
}

func (JournalEntryModel) TableName() string { return "journal_entries" }

type JournalLineModel struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID      uuid.UUID  `gorm:"type:uuid;index;not null"`
	EntryID    uuid.UUID  `gorm:"type:uuid;index;not null"`
	AccountID  uuid.UUID  `gorm:"type:uuid;not null"`
	Debit      float64    `gorm:"not null;default:0"`
	Credit     float64    `gorm:"not null;default:0"`
	BaseAmount float64    `gorm:"not null;default:0"`
	PartyID    *uuid.UUID `gorm:"type:uuid"`
	Memo       string     `gorm:"not null;default:''"`
	LineNo     int        `gorm:"not null;default:0"`
}

func (JournalLineModel) TableName() string { return "journal_lines" }

type OutboxModel struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID         uuid.UUID `gorm:"type:uuid;index;not null"`
	ReferenceType string    `gorm:"not null"`
	ReferenceID   uuid.UUID `gorm:"type:uuid;not null"`
	SourceEvent   string    `gorm:"not null"`
	Payload       []byte    `gorm:"type:jsonb"`
	Status        string    `gorm:"not null;default:pending"`
	Attempts      int       `gorm:"not null;default:0"`
	MaxAttempts   int       `gorm:"not null;default:10"`
	NextRetry     *time.Time
	LastError     string `gorm:"not null;default:''"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (OutboxModel) TableName() string { return "ledger_outbox" }
