package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Bicycle struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
	CustomerID      *uuid.UUID
	CustomerName    string
	FrameNumber     string
	Brand           string
	Model           string
	BikeType        string
	Size            string
	WheelSizeInches int
	Color           string
	EbikeNotes      string
	Notes           string
	IsFavorite      bool
	Tags            []string
	ArchivedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (b Bicycle) DisplayLabel() string {
	label := strings.TrimSpace(strings.Join([]string{
		strings.TrimSpace(b.Brand),
		strings.TrimSpace(b.Model),
	}, " "))
	if label != "" {
		return label
	}
	if frame := strings.TrimSpace(b.FrameNumber); frame != "" {
		return frame
	}
	return "Bicicleta"
}
