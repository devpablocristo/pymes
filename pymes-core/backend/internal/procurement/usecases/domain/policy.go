package domain

import (
	"time"

	"github.com/google/uuid"
)

// ProcurementPolicy regla CEL por tenant (mapea a kernel.Policy al evaluar).
type ProcurementPolicy struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
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
