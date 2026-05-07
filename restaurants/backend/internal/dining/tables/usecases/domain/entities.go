package domain

import (
	"time"

	"github.com/google/uuid"
)

type DiningTable struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	AreaID     uuid.UUID
	Code       string
	Label      string
	Capacity   int
	Status     string
	Notes      string
	IsFavorite bool
	Tags       []string
	Metadata   map[string]any
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}
