package domain

import (
	"time"

	"github.com/google/uuid"
)

type StaffMember struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	DisplayName string
	Role        string
	Color       string
	IsActive    bool
	Notes       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
