package domain

import (
	"time"

	"github.com/google/uuid"
)

type SalonService struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
	Code            string
	Name            string
	Description     string
	Category        string
	DurationMinutes int
	BasePrice       float64
	Currency        string
	TaxRate         float64
	LinkedProductID *uuid.UUID
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
