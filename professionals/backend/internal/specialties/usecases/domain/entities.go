package domain

import (
	"time"

	"github.com/google/uuid"
)

type Specialty struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	Code        string
	Name        string
	Description string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
