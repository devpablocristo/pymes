package domain

import (
	"time"

	"github.com/google/uuid"
)

type Service struct {
	ID                     uuid.UUID
	OrgID                  uuid.UUID
	Code                   string
	Name                   string
	Description            string
	CategoryCode           string
	SalePrice              float64
	CostPrice              float64
	TaxRate                *float64
	Currency               string
	DefaultDurationMinutes *int
	Tags                   []string
	Metadata               map[string]any
	CreatedAt              time.Time
	UpdatedAt              time.Time
	DeletedAt              *time.Time
}
