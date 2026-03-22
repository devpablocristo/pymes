package domain

import (
	"time"

	"github.com/google/uuid"
)

type DiningArea struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	Name      string
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
}
