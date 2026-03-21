package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type RequestStatus string

const (
	StatusDraft            RequestStatus = "draft"
	StatusSubmitted        RequestStatus = "submitted"
	StatusPendingApproval  RequestStatus = "pending_approval"
	StatusApproved           RequestStatus = "approved"
	StatusRejected           RequestStatus = "rejected"
	StatusCancelled          RequestStatus = "cancelled"
)

type ProcurementRequest struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
	RequesterActor  string
	Title           string
	Description     string
	Category        string
	Status          RequestStatus
	EstimatedTotal  float64
	Currency        string
	EvaluationJSON  json.RawMessage
	PurchaseID      *uuid.UUID
	Lines           []RequestLine
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ArchivedAt      *time.Time
}

type RequestLine struct {
	ID                uuid.UUID
	RequestID         uuid.UUID
	Description       string
	ProductID         *uuid.UUID
	Quantity          float64
	UnitPriceEstimate float64
	SortOrder         int
}
