package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type RequestLine struct {
	ID                uuid.UUID  `json:"id,omitempty"`
	Description       string     `json:"description"`
	ProductID         *uuid.UUID `json:"product_id"`
	Quantity          float64    `json:"quantity"`
	UnitPriceEstimate float64    `json:"unit_price_estimate"`
}

type CreateRequest struct {
	Title          string        `json:"title"`
	Description    string        `json:"description"`
	Category       string        `json:"category"`
	EstimatedTotal float64       `json:"estimated_total"`
	Currency       string        `json:"currency"`
	Lines          []RequestLine `json:"lines"`
}

type UpdateRequest struct {
	Title          string        `json:"title"`
	Description    string        `json:"description"`
	Category       string        `json:"category"`
	EstimatedTotal float64       `json:"estimated_total"`
	Currency       string        `json:"currency"`
	Lines          []RequestLine `json:"lines"`
}

// PolicyResponse política de procurement (CEL / governance).
type PolicyResponse struct {
	ID           uuid.UUID `json:"id"`
	OrgID        uuid.UUID `json:"org_id"`
	Name         string    `json:"name"`
	Expression   string    `json:"expression"`
	Effect       string    `json:"effect"`
	Priority     int       `json:"priority"`
	Mode         string    `json:"mode"`
	Enabled      bool      `json:"enabled"`
	ActionFilter string    `json:"action_filter"`
	SystemFilter string    `json:"system_filter"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreatePolicyRequest struct {
	Name         string `json:"name"`
	Expression   string `json:"expression"`
	Effect       string `json:"effect"`
	Priority     int    `json:"priority"`
	Mode         string `json:"mode"`
	Enabled      bool   `json:"enabled"`
	ActionFilter string `json:"action_filter"`
	SystemFilter string `json:"system_filter"`
}

type UpdatePolicyRequest struct {
	Name         string `json:"name"`
	Expression   string `json:"expression"`
	Effect       string `json:"effect"`
	Priority     int    `json:"priority"`
	Mode         string `json:"mode"`
	Enabled      bool   `json:"enabled"`
	ActionFilter string `json:"action_filter"`
	SystemFilter string `json:"system_filter"`
}

type RequestResponse struct {
	ID             uuid.UUID       `json:"id"`
	OrgID          uuid.UUID       `json:"org_id"`
	RequesterActor string          `json:"requester_actor"`
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	Category       string          `json:"category"`
	Status         string          `json:"status"`
	EstimatedTotal float64         `json:"estimated_total"`
	Currency       string          `json:"currency"`
	EvaluationJSON json.RawMessage `json:"evaluation,omitempty"`
	PurchaseID     *uuid.UUID      `json:"purchase_id,omitempty"`
	Lines          []RequestLine   `json:"lines"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	ArchivedAt     *time.Time      `json:"archived_at,omitempty"`
}
