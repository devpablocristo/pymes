// Package dto contains HTTP payloads owned by the identity handler adapter.
package dto

import "time"

// WebhookEvent is the verified HTTP event before it enters the use case.
type WebhookEvent struct {
	ID         string
	Type       string
	OccurredAt time.Time
	Payload    []byte
}
