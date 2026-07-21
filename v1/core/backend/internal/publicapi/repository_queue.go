package publicapi

import (
	"context"

	"github.com/google/uuid"

	schedulingdomain "github.com/devpablocristo/platform/features/scheduling/go/domain"
	schedulingpublic "github.com/devpablocristo/platform/features/scheduling/go/publicapi"
)

func (r *Repository) ListPublicQueues(ctx context.Context, tenantID uuid.UUID, branchID *uuid.UUID) ([]schedulingpublic.QueueSummary, error) {
	if r.scheduling == nil {
		return nil, nil
	}
	return r.scheduling.ListQueues(ctx, tenantID, branchID)
}

func (r *Repository) CreatePublicQueueTicket(ctx context.Context, tenantID, queueID uuid.UUID, payload map[string]any) (schedulingpublic.QueueTicket, schedulingpublic.QueuePosition, error) {
	if r.scheduling == nil {
		return schedulingpublic.QueueTicket{}, schedulingpublic.QueuePosition{}, ErrInvalidInput
	}
	partyID, err := uuidPtrFromPayload(payload, "party_id")
	if err != nil {
		return schedulingpublic.QueueTicket{}, schedulingpublic.QueuePosition{}, ErrInvalidInput
	}
	item, err := r.scheduling.IssueQueueTicket(ctx, tenantID, "public-api", schedulingdomain.CreateQueueTicketInput{
		QueueID:        queueID,
		PartyID:        partyID,
		CustomerName:   firstStringFromPayload(payload, "customer_name", "party_name"),
		CustomerPhone:  firstStringFromPayload(payload, "customer_phone", "party_phone"),
		CustomerEmail:  firstStringFromPayload(payload, "customer_email"),
		Priority:       intValueFromPayload(payload, "priority"),
		Source:         schedulingdomain.QueueTicketSource(firstStringFromPayload(payload, "source")),
		IdempotencyKey: firstStringFromPayload(payload, "idempotency_key"),
		Notes:          firstStringFromPayload(payload, "notes"),
		Metadata:       ensureMap(payload["metadata"]),
	})
	if err != nil {
		return schedulingpublic.QueueTicket{}, schedulingpublic.QueuePosition{}, mapSchedulingErr(err)
	}
	position, err := r.scheduling.GetQueueTicketPosition(ctx, tenantID, queueID, item.ID)
	if err != nil {
		return schedulingpublic.QueueTicket{}, schedulingpublic.QueuePosition{}, mapSchedulingErr(err)
	}
	return item, position, nil
}

func (r *Repository) GetPublicQueueTicketPosition(ctx context.Context, tenantID, queueID, ticketID uuid.UUID) (schedulingpublic.QueuePosition, error) {
	if r.scheduling == nil {
		return schedulingpublic.QueuePosition{}, ErrInvalidInput
	}
	position, err := r.scheduling.GetQueueTicketPosition(ctx, tenantID, queueID, ticketID)
	if err != nil {
		return schedulingpublic.QueuePosition{}, mapSchedulingErr(err)
	}
	return position, nil
}

func (r *Repository) JoinWaitlist(ctx context.Context, tenantID uuid.UUID, payload map[string]any) (schedulingpublic.WaitlistEntry, error) {
	if r.scheduling == nil {
		return schedulingpublic.WaitlistEntry{}, ErrInvalidInput
	}
	branchID, err := uuidValueFromPayload(payload, "branch_id")
	if err != nil {
		return schedulingpublic.WaitlistEntry{}, ErrInvalidInput
	}
	serviceID, err := uuidValueFromPayload(payload, "service_id")
	if err != nil {
		return schedulingpublic.WaitlistEntry{}, ErrInvalidInput
	}
	resourceID, err := uuidPtrFromPayload(payload, "resource_id")
	if err != nil {
		return schedulingpublic.WaitlistEntry{}, ErrInvalidInput
	}
	partyID, err := uuidPtrFromPayload(payload, "party_id")
	if err != nil {
		return schedulingpublic.WaitlistEntry{}, ErrInvalidInput
	}
	requestedStartAt, err := timeValueFromPayload(payload, "requested_start_at")
	if err != nil {
		return schedulingpublic.WaitlistEntry{}, ErrInvalidInput
	}
	item, err := r.scheduling.JoinWaitlist(ctx, tenantID, "public-api", schedulingdomain.CreateWaitlistInput{
		BranchID:         branchID,
		ServiceID:        serviceID,
		ResourceID:       resourceID,
		PartyID:          partyID,
		CustomerName:     firstStringFromPayload(payload, "customer_name", "party_name"),
		CustomerPhone:    firstStringFromPayload(payload, "customer_phone", "party_phone"),
		CustomerEmail:    firstStringFromPayload(payload, "customer_email"),
		RequestedStartAt: requestedStartAt.UTC(),
		Source:           schedulingdomain.WaitlistSource(firstStringFromPayload(payload, "source")),
		IdempotencyKey:   firstStringFromPayload(payload, "idempotency_key"),
		Notes:            firstStringFromPayload(payload, "notes"),
		Metadata:         ensureMap(payload["metadata"]),
	})
	if err != nil {
		return schedulingpublic.WaitlistEntry{}, mapSchedulingErr(err)
	}
	return item, nil
}
