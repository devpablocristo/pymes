package usecases

import (
	"context"
	"fmt"

	commerce "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
	identity "github.com/devpablocristo/pymes/v3/backend/internal/identity/domain"
	identityusecases "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases"
)

func requireEventOrganization(event commerce.Event, aggregateOrganizationID string) error {
	if event.OrganizationID == "" || aggregateOrganizationID == "" ||
		event.OrganizationID != aggregateOrganizationID {
		return fmt.Errorf(
			"OUTBOX_ORGANIZATION_MISMATCH: event %s organization %q, aggregate organization %q",
			event.ID,
			event.OrganizationID,
			aggregateOrganizationID,
		)
	}
	return nil
}

func persistedSourceVersion(origin commerce.OriginMetadata) int {
	if origin.SourceVersion > 0 {
		return origin.SourceVersion
	}
	return 1
}

func persistedCorrelationID(origin commerce.OriginMetadata, fallback string) string {
	if origin.CorrelationID != "" {
		return origin.CorrelationID
	}
	return fallback
}

// internalDeliveryContext replaces any caller identity with the immutable
// origin stored for this delivery. The principal is tenant-scoped so the
// internal token source cannot attribute a cross-organization request to the
// delegated actor.
func internalDeliveryContext(
	ctx context.Context,
	organizationID, actorRef, requestID, correlationID string,
) context.Context {
	if requestID == "" {
		requestID = correlationID
	}
	if correlationID == "" {
		correlationID = requestID
	}
	ctx = identityusecases.WithRequestMetadata(ctx, identityusecases.RequestMetadata{
		RequestID:     requestID,
		CorrelationID: correlationID,
	})
	ctx = identityusecases.WithPrincipal(ctx, identity.Principal{
		OrganizationID:     organizationID,
		ActorID:            actorRef,
		Role:               identity.RoleMember,
		OrganizationStatus: "ready",
		MembershipStatus:   "active",
	})
	return identityusecases.WithDelegatedActor(ctx, actorRef)
}

func outboxDeliveryContext(ctx context.Context, event commerce.Event) context.Context {
	requestID := event.RequestID
	if requestID == "" {
		requestID = event.ID
	}
	return internalDeliveryContext(
		ctx,
		event.OrganizationID,
		event.ActorRef,
		requestID,
		event.CorrelationID,
	)
}

func aggregateDeliveryContext(
	ctx context.Context,
	organizationID string,
	origin commerce.OriginMetadata,
	requestID, correlationFallback string,
) context.Context {
	return internalDeliveryContext(
		ctx,
		organizationID,
		origin.ActorRef,
		requestID,
		persistedCorrelationID(origin, correlationFallback),
	)
}
