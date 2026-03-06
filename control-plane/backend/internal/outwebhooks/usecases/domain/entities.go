package domain

import (
	"time"

	"github.com/google/uuid"
)

type Endpoint struct {
	ID        uuid.UUID `json:"id"`
	OrgID     uuid.UUID `json:"org_id"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret,omitempty"`
	Events    []string  `json:"events"`
	IsActive  bool      `json:"is_active"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Delivery struct {
	ID           uuid.UUID      `json:"id"`
	EndpointID   uuid.UUID      `json:"endpoint_id"`
	EventType    string         `json:"event_type"`
	Payload      map[string]any `json:"payload"`
	StatusCode   *int           `json:"status_code,omitempty"`
	ResponseBody string         `json:"response_body"`
	Attempts     int            `json:"attempts"`
	NextRetry    *time.Time     `json:"next_retry,omitempty"`
	DeliveredAt  *time.Time     `json:"delivered_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}
