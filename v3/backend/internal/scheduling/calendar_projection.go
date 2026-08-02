// architecture:adapter external
package scheduling

import (
	"time"

	projectionhelpers "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/calendar_projection/helpers"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
)

type CalendarProjectionAdapter struct {
	now func() time.Time
}

func NewCalendarProjectionAdapter() *CalendarProjectionAdapter {
	return &CalendarProjectionAdapter{now: time.Now}
}

func (adapter *CalendarProjectionAdapter) Upsert(
	metadata domain.CommandMetadata,
	booking domain.Booking,
) domain.Event {
	return projectionhelpers.Event(
		metadata,
		booking,
		"upsert",
		uuid.New(),
		adapter.now(),
	)
}

func (adapter *CalendarProjectionAdapter) Delete(
	metadata domain.CommandMetadata,
	booking domain.Booking,
) domain.Event {
	return projectionhelpers.Event(
		metadata,
		booking,
		"delete",
		uuid.New(),
		adapter.now(),
	)
}
