package domain

import (
	"time"

	"github.com/google/uuid"
)

// ProcurementPolicy regla CEL por organización (mapea a kernel.Policy al evaluar).
type ProcurementPolicy struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	Name         string
	Expression   string
	Effect       string
	Priority     int
	Mode         string
	Enabled      bool
	ActionFilter string
	SystemFilter string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
