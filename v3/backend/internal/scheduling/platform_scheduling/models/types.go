package models

import (
	"time"

	platformdomain "github.com/devpablocristo/platform/features/scheduling/go/domain"
	"github.com/google/uuid"
)

// ResourceSlotSet is the provider-specific representation used only inside
// the Platform adapter boundary.
type ResourceSlotSet struct {
	ResourceID uuid.UUID
	Slots      []platformdomain.TimeSlot
}

type LocalWindow struct {
	StartAt time.Time
	EndAt   time.Time
}
