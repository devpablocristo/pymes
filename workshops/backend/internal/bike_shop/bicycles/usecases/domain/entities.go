package domain

import (
	"time"

	"github.com/google/uuid"
)

type Bicycle struct {
	ID               uuid.UUID
	OrgID            uuid.UUID
	CustomerID       *uuid.UUID
	CustomerName     string
	FrameNumber      string
	Make             string
	Model            string
	BikeType         string
	Size             string
	WheelSizeInches  int
	Color            string
	EbikeNotes       string
	Notes            string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
