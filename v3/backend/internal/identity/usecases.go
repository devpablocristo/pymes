package identity

import (
	"context"
	"time"

	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"
)

type principalContextKey struct{}
type delegatedActorContextKey struct{}
type requestMetadataContextKey struct{}

type RequestMetadata struct {
	RequestID     string
	CorrelationID string
}

// WebhookInbox is the outbound port consumed by the idempotent webhook use case.
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

func WithPrincipal(ctx context.Context, principal identitydomain.Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (identitydomain.Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(identitydomain.Principal)
	return principal, ok
}

func WithDelegatedActor(ctx context.Context, actorID string) context.Context {
	return context.WithValue(ctx, delegatedActorContextKey{}, actorID)
}

func DelegatedActorFromContext(ctx context.Context) (string, bool) {
	actorID, ok := ctx.Value(delegatedActorContextKey{}).(string)
	return actorID, ok && actorID != ""
}

func WithRequestMetadata(ctx context.Context, metadata RequestMetadata) context.Context {
	return context.WithValue(ctx, requestMetadataContextKey{}, metadata)
}

func RequestMetadataFromContext(ctx context.Context) (RequestMetadata, bool) {
	metadata, ok := ctx.Value(requestMetadataContextKey{}).(RequestMetadata)
	if !ok || metadata.RequestID == "" || metadata.CorrelationID == "" {
		return RequestMetadata{}, false
	}
	return metadata, true
}
