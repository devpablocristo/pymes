package domain

import (
	"time"

	"github.com/google/uuid"
)

type ServiceLink struct {
	ID                uuid.UUID
	OrgID             uuid.UUID
	ProfileID         uuid.UUID
	ProductID         uuid.UUID
	PublicDescription string
	DisplayOrder      int
	IsFeatured        bool
	Metadata          map[string]any
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
