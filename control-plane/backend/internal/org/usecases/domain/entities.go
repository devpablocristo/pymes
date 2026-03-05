package domain

import (
	"time"

	"github.com/google/uuid"
)

type Organization struct {
	ID         uuid.UUID `json:"id"`
	ExternalID string    `json:"external_id,omitempty"`
	Name       string    `json:"name"`
	Slug       string    `json:"slug,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
