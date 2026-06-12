package domain

import (
	"time"

	"github.com/google/uuid"
)

type DiningArea struct {
	ID         uuid.UUID
	OrgID   uuid.UUID
	Name       string
	SortOrder  int
	IsFavorite bool
	Tags       []string
	Metadata   map[string]any
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}
