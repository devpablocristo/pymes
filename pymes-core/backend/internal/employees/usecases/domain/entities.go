package domain

import (
	"time"

	"github.com/google/uuid"
)

type EmployeeStatus string

const (
	EmployeeStatusActive     EmployeeStatus = "active"
	EmployeeStatusInactive   EmployeeStatus = "inactive"
	EmployeeStatusTerminated EmployeeStatus = "terminated"
)

type Employee struct {
	ID         uuid.UUID      `json:"id"`
	OrgID      uuid.UUID      `json:"org_id"`
	FirstName  string         `json:"first_name"`
	LastName   string         `json:"last_name"`
	Email      string         `json:"email"`
	Phone      string         `json:"phone"`
	Position   string         `json:"position"`
	Status     EmployeeStatus `json:"status"`
	HireDate   *time.Time     `json:"hire_date,omitempty"`
	EndDate    *time.Time     `json:"end_date,omitempty"`
	UserID     *uuid.UUID     `json:"user_id,omitempty"`
	Notes      string         `json:"notes"`
	IsFavorite bool           `json:"is_favorite"`
	Tags       []string       `json:"tags"`
	CreatedBy  string         `json:"created_by"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	ArchivedAt *time.Time     `json:"archived_at,omitempty"`
}
