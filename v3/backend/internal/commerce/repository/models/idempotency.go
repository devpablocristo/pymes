package models

import (
	"encoding/json"
	"time"
)

type IdempotencyRecord struct {
	Operation     string
	SourceID      string
	PayloadHash   string
	Key           string
	SourceVersion int
	Response      json.RawMessage
	CompletedAt   *time.Time
}

type PendingApplication struct {
	ID         string
	Kind       string
	DocumentID string
	Amount     string
	Currency   string
}
