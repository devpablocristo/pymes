package domain

import (
	"time"

	"github.com/google/uuid"
)

type Exam struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	PatientName     string
	PatientDocument string
	EmployerName    string
	ClientName      string
	PaymentMethod   string
	ExamType        string
	Status          string
	ScheduledAt     *time.Time
	CompletedAt     *time.Time
	Result          string
	Notes           string
	IsFavorite      bool
	Tags            []string
	ImageURLs       []string
	CreatedBy       string
	UpdatedBy       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}
