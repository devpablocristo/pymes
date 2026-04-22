package domain

import (
	"time"

	"github.com/google/uuid"
)

type DiningTable struct {
	ID         uuid.UUID
	OrgID      uuid.UUID
	AreaID     uuid.UUID
	Code       string
	Label      string
	Capacity   int
	Status     string
	Notes      string
	IsFavorite bool
	Tags       []string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
