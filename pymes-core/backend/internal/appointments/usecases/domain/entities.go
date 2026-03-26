package domain

import (
	"time"

	"github.com/google/uuid"
)

type Appointment struct {
	ID            uuid.UUID      `json:"id"`
	OrgID         uuid.UUID      `json:"org_id"`
	CustomerID    *uuid.UUID     `json:"customer_id,omitempty"`
	CustomerName  string         `json:"customer_name"`
	CustomerPhone string         `json:"customer_phone"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	Status        string         `json:"status"`
	StartAt       time.Time      `json:"start_at"`
	EndAt         time.Time      `json:"end_at"`
	Duration      int            `json:"duration"`
	Location      string         `json:"location"`
	AssignedTo    string         `json:"assigned_to"`
	Color         string         `json:"color"`
	Notes         string         `json:"notes"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CreatedBy     string         `json:"created_by,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	ArchivedAt    *time.Time     `json:"archived_at,omitempty"`
}
