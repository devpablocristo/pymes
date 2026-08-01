package models

import (
	"encoding/json"
	"time"
)

type OutboxEvent struct {
	ID             string
	OrganizationID string
	Topic          string
	Payload        json.RawMessage
	PayloadHash    string
	IdempotencyKey string
	RequestID      string
	ActorRef       string
	SourceVersion  int
	SnapshotDigest string
	CorrelationID  string
	AvailableAt    time.Time
	Attempts       int
	LeaseToken     string
	LeaseExpiresAt time.Time
}
