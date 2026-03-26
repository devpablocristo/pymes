package domain

import (
	"time"

	"github.com/google/uuid"
)

type Vehicle struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	CustomerID   *uuid.UUID
	CustomerName string
	LicensePlate string
	VIN          string
	Make         string
	Model        string
	Year         int
	Kilometers   int
	Color        string
	Notes        string
	ArchivedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
