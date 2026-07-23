package iamoutbox

import (
	"context"
	"errors"
	"fmt"
	"strings"

	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
)

const defaultInvitationExpiryDays = 7

type clerkProvisioner interface {
	ListOrganizations(context.Context, clerk.ListInput) ([]clerk.Organization, error)
	CreateOrganization(context.Context, clerk.OrganizationInput) (clerk.Organization, error)
	UpdateOrganization(context.Context, string, clerk.OrganizationInput) (clerk.Organization, error)
	ListOrgInvitations(context.Context, string, clerk.OrgInvitationListInput) ([]clerk.Invitation, error)
	CreateOrgInvitation(context.Context, clerk.OrgInvitationInput) (clerk.Invitation, error)
}

type finalizer interface {
	Finalize(context.Context, Finalization) error
}

// Finalization contains only provider identifiers and public invitation
// metadata. No Clerk ticket or token is accepted or persisted.
type Finalization struct {
	MessageID            string
	Event                ProvisionOrganizationEvent
	ProviderOrganization clerk.Organization
	ProviderInvitation   clerk.Invitation
}

// Processor is the topic-aware outbox publisher for IAM provisioning.
type Processor struct {
	clerk     clerkProvisioner
	finalizer finalizer
}

func NewProcessor(provider clerkProvisioner, completion finalizer) (*Processor, error) {
	if provider == nil {
		return nil, errors.New("iam outbox: Clerk provisioner is required")
	}
	if completion == nil {
		return nil, errors.New("iam outbox: finalizer is required")
	}
	return &Processor{clerk: provider, finalizer: completion}, nil
}

// Publish implements platform/outbox Publisher. Provider failures and local
// transaction failures are deliberately returned so Dispatcher schedules a
// durable retry.
func (processor *Processor) Publish(ctx context.Context, publication platformoutbox.Publication) error {
	if processor == nil || processor.clerk == nil || processor.finalizer == nil {
		return errors.New("iam outbox: processor is not configured")
	}
	if publication.Topic != ProvisionOrganizationTopic {
		return fmt.Errorf("%w: %q", ErrUnsupportedTopic, publication.Topic)
	}

	event, err := decodeProvisionOrganizationEvent(publication.Payload)
	if err != nil {
		return err
	}
	if strings.TrimSpace(publication.MessageID) == "" {
		return fmt.Errorf("%w: message ID is required", ErrInvalidEvent)
	}
	expectedKey := "iam.provision-org:" + event.Slug
	if publication.IdempotencyKey != expectedKey {
		return fmt.Errorf(
			"%w: idempotency key %q does not match %q",
			ErrInvalidEvent,
			publication.IdempotencyKey,
			expectedKey,
		)
	}

	organization, err := processor.reconcileOrganization(ctx, event)
	if err != nil {
		return fmt.Errorf("reconcile Clerk organization %q: %w", event.Slug, err)
	}
	invitation, err := processor.reconcileOwnerInvitation(ctx, event, organization.ID)
	if err != nil {
		return fmt.Errorf("reconcile Clerk owner invitation for %q: %w", event.Slug, err)
	}

	if err := processor.finalizer.Finalize(ctx, Finalization{
		MessageID:            publication.MessageID,
		Event:                event,
		ProviderOrganization: organization,
		ProviderInvitation:   invitation,
	}); err != nil {
		return fmt.Errorf("finalize organization provisioning %q: %w", event.Slug, err)
	}
	return nil
}

func (processor *Processor) reconcileOrganization(
	ctx context.Context,
	event ProvisionOrganizationEvent,
) (clerk.Organization, error) {
	organizations, err := processor.clerk.ListOrganizations(ctx, clerk.ListInput{Limit: 100})
	if err != nil {
		return clerk.Organization{}, err
	}
	organization, found, err := exactOrganizationBySlug(organizations, event.Slug)
	if err != nil {
		return clerk.Organization{}, err
	}
	if found {
		if organization.Name != event.Name {
			organization, err = processor.clerk.UpdateOrganization(
				ctx,
				organization.ID,
				clerk.OrganizationInput{Name: event.Name, Slug: event.Slug},
			)
			if err != nil {
				return clerk.Organization{}, err
			}
		}
		return validateProviderOrganization(organization, event)
	}

	organization, createErr := processor.clerk.CreateOrganization(
		ctx,
		clerk.OrganizationInput{Name: event.Name, Slug: event.Slug},
	)
	if createErr == nil {
		return validateProviderOrganization(organization, event)
	}

	// The create may have reached Clerk even when the response was lost, or a
	// concurrent retry may have won. Resolve the natural key once more before
	// returning the provider error to Dispatcher.
	organizations, reconcileErr := processor.clerk.ListOrganizations(ctx, clerk.ListInput{Limit: 100})
	if reconcileErr == nil {
		organization, found, findErr := exactOrganizationBySlug(organizations, event.Slug)
		if findErr != nil {
			return clerk.Organization{}, findErr
		}
		if found {
			return validateProviderOrganization(organization, event)
		}
	}
	return clerk.Organization{}, createErr
}

func (processor *Processor) reconcileOwnerInvitation(
	ctx context.Context,
	event ProvisionOrganizationEvent,
	providerOrganizationID string,
) (clerk.Invitation, error) {
	listInput := clerk.OrgInvitationListInput{
		ListInput: clerk.ListInput{Limit: 100},
		Statuses:  []string{"pending"},
		Email:     event.OwnerEmail,
	}
	invitations, err := processor.clerk.ListOrgInvitations(ctx, providerOrganizationID, listInput)
	if err != nil {
		return clerk.Invitation{}, err
	}
	invitation, found, err := exactPendingInvitation(invitations, providerOrganizationID, event)
	if err != nil {
		return clerk.Invitation{}, err
	}
	if found {
		return invitation, nil
	}

	invitation, createErr := processor.clerk.CreateOrgInvitation(ctx, clerk.OrgInvitationInput{
		ProviderOrgID: providerOrganizationID,
		Email:         event.OwnerEmail,
		Role:          event.ProviderRole,
		ExpiresInDays: defaultInvitationExpiryDays,
	})
	if createErr == nil {
		return validateProviderInvitation(invitation, providerOrganizationID, event)
	}

	// As with organization creation, recover a successful provider side effect
	// whose response was lost before asking Dispatcher to retry.
	invitations, reconcileErr := processor.clerk.ListOrgInvitations(ctx, providerOrganizationID, listInput)
	if reconcileErr == nil {
		invitation, found, findErr := exactPendingInvitation(
			invitations,
			providerOrganizationID,
			event,
		)
		if findErr != nil {
			return clerk.Invitation{}, findErr
		}
		if found {
			return invitation, nil
		}
	}
	return clerk.Invitation{}, createErr
}

func exactOrganizationBySlug(
	organizations []clerk.Organization,
	slug string,
) (clerk.Organization, bool, error) {
	match := clerk.Organization{}
	found := false
	for _, organization := range organizations {
		if strings.TrimSpace(organization.Slug) != slug {
			continue
		}
		if found && strings.TrimSpace(match.ID) != strings.TrimSpace(organization.ID) {
			return clerk.Organization{}, false, fmt.Errorf(
				"multiple Clerk organizations use slug %q",
				slug,
			)
		}
		match = organization
		found = true
	}
	return match, found, nil
}

func exactPendingInvitation(
	invitations []clerk.Invitation,
	providerOrganizationID string,
	event ProvisionOrganizationEvent,
) (clerk.Invitation, bool, error) {
	match := clerk.Invitation{}
	found := false
	for _, invitation := range invitations {
		if strings.ToLower(strings.TrimSpace(invitation.Email)) != event.OwnerEmail ||
			strings.TrimSpace(invitation.Status) != "pending" {
			continue
		}
		if strings.TrimSpace(invitation.Role) != event.ProviderRole {
			return clerk.Invitation{}, false, fmt.Errorf(
				"pending invitation for %q has provider role %q",
				event.OwnerEmail,
				invitation.Role,
			)
		}
		if found && strings.TrimSpace(match.ID) != strings.TrimSpace(invitation.ID) {
			return clerk.Invitation{}, false, fmt.Errorf(
				"multiple pending invitations exist for %q",
				event.OwnerEmail,
			)
		}
		match = invitation
		found = true
	}
	if !found {
		return clerk.Invitation{}, false, nil
	}
	validated, err := validateProviderInvitation(match, providerOrganizationID, event)
	if err != nil {
		return clerk.Invitation{}, false, err
	}
	return validated, true, nil
}

func validateProviderOrganization(
	organization clerk.Organization,
	event ProvisionOrganizationEvent,
) (clerk.Organization, error) {
	organization.ID = strings.TrimSpace(organization.ID)
	organization.Name = strings.TrimSpace(organization.Name)
	organization.Slug = strings.TrimSpace(organization.Slug)
	if organization.ID == "" {
		return clerk.Organization{}, errors.New("Clerk organization ID is empty")
	}
	if organization.Slug != event.Slug {
		return clerk.Organization{}, fmt.Errorf(
			"Clerk organization slug %q does not match %q",
			organization.Slug,
			event.Slug,
		)
	}
	if organization.Name != event.Name {
		return clerk.Organization{}, fmt.Errorf(
			"Clerk organization name %q does not match %q",
			organization.Name,
			event.Name,
		)
	}
	return organization, nil
}

func validateProviderInvitation(
	invitation clerk.Invitation,
	providerOrganizationID string,
	event ProvisionOrganizationEvent,
) (clerk.Invitation, error) {
	invitation.ID = strings.TrimSpace(invitation.ID)
	invitation.OrganizationID = strings.TrimSpace(invitation.OrganizationID)
	invitation.Email = strings.ToLower(strings.TrimSpace(invitation.Email))
	invitation.Role = strings.TrimSpace(invitation.Role)
	invitation.Status = strings.TrimSpace(invitation.Status)
	if invitation.ID == "" {
		return clerk.Invitation{}, errors.New("Clerk invitation ID is empty")
	}
	if invitation.OrganizationID != providerOrganizationID {
		return clerk.Invitation{}, fmt.Errorf(
			"Clerk invitation organization %q does not match %q",
			invitation.OrganizationID,
			providerOrganizationID,
		)
	}
	if invitation.Email != event.OwnerEmail {
		return clerk.Invitation{}, fmt.Errorf(
			"Clerk invitation email %q does not match %q",
			invitation.Email,
			event.OwnerEmail,
		)
	}
	if invitation.Role != event.ProviderRole || invitation.Status != "pending" {
		return clerk.Invitation{}, fmt.Errorf(
			"Clerk invitation must be pending with role %q",
			event.ProviderRole,
		)
	}
	if invitation.ExpiresAt == nil || invitation.ExpiresAt.IsZero() {
		return clerk.Invitation{}, errors.New("Clerk invitation expiry is empty")
	}
	expiresAt := invitation.ExpiresAt.UTC()
	invitation.ExpiresAt = &expiresAt
	return invitation, nil
}
