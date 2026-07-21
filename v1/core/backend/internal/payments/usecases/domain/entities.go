package domain

import (
	"time"

	"github.com/google/uuid"
)

type Payment struct {
	ID            uuid.UUID  `json:"id"`
	OrgID         uuid.UUID  `json:"org_id"`
	ReferenceType string     `json:"reference_type"`
	ReferenceID   uuid.UUID  `json:"reference_id"`
	Method        string     `json:"method"`
	Amount        float64    `json:"amount"`
	Notes         string     `json:"notes"`
	ReceivedAt    time.Time  `json:"received_at"`
	IsFavorite    bool       `json:"is_favorite"`
	Tags          []string   `json:"tags"`
	ArchivedAt    *time.Time `json:"archived_at,omitempty"`
	CreatedBy     string     `json:"created_by,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}
