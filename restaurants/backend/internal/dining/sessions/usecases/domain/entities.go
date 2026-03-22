package domain

import (
	"time"

	"github.com/google/uuid"
)

type TableSession struct {
	ID         uuid.UUID
	OrgID      uuid.UUID
	TableID    uuid.UUID
	GuestCount int
	PartyLabel string
	Notes      string
	OpenedAt   time.Time
	ClosedAt   *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type TableSessionListItem struct {
	TableSession
	TableCode string
	AreaName  string
}
