package domain

import (
	"time"

	"github.com/google/uuid"
)

type Product struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	SKU         string
	Name        string
	Description string
	Unit        string
	Price       float64
	Currency    string
	CostPrice   float64
	TaxRate     *float64
	ImageURL    string
	TrackStock  bool
	IsActive    bool
	Tags        []string
	Metadata    map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}
