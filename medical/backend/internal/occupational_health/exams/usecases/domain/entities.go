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
	ExamType        string
	Status          string
	ScheduledAt     *time.Time
	CompletedAt     *time.Time
	Result          string
	Notes           string
	CreatedBy       string
	UpdatedBy       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}
