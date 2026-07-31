package usecases

import (
	"context"
	"time"
)

// WebhookInbox separates provider verification from durable, idempotent local
// projection. Adapters own HTTP and PostgreSQL; this use-case owns the rule
// that the same Clerk event is processed at most once.
type WebhookInbox interface {
	Receive(context.Context, Event) (duplicate bool, err error)
}
type Event struct {
	ID, Type   string
	OccurredAt time.Time
	Payload    []byte
}
type ReceiveWebhook struct{ Inbox WebhookInbox }

func (u ReceiveWebhook) Execute(ctx context.Context, event Event) (bool, error) {
	return u.Inbox.Receive(ctx, event)
}
