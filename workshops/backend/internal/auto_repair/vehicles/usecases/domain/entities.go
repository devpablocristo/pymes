package domain

import (
	"time"

	"github.com/google/uuid"
)

type Vehicle struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
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
	IsFavorite   bool
	Tags         []string
	ArchivedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
