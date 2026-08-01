package models

import (
	"encoding/json"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
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

func (event OutboxEvent) Domain() domain.OutboxEvent {
	return domain.OutboxEvent{
		ID: event.ID, OrganizationID: event.OrganizationID,
		Topic: event.Topic, Payload: event.Payload,
		PayloadHash: event.PayloadHash, IdempotencyKey: event.IdempotencyKey,
		RequestID: event.RequestID, ActorRef: event.ActorRef,
		SourceVersion: event.SourceVersion, SnapshotDigest: event.SnapshotDigest,
		CorrelationID: event.CorrelationID, AvailableAt: event.AvailableAt,
		Attempts: event.Attempts, LeaseToken: event.LeaseToken,
		LeaseExpiresAt: event.LeaseExpiresAt,
	}
}
