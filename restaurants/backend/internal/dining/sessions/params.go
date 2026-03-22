package sessions

import "github.com/google/uuid"

// ListParams filtra el historial de sesiones de mesa.
type ListParams struct {
	OrgID    uuid.UUID
	OpenOnly bool
	TableID  *uuid.UUID
}
