package iamsync

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"sync"
	"time"

	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
)

type clerkManager interface {
	GetOrganization(context.Context, string) (clerk.Organization, error)
	UpdateOrganization(context.Context, string, clerk.OrganizationInput) (clerk.Organization, error)
	GetOrgMembership(context.Context, string, string) (clerk.OrganizationMembership, bool, error)
	CreateOrgMembership(context.Context, clerk.OrgMembershipInput) error
	UpdateOrgMembership(context.Context, clerk.OrgMembershipInput) error
	RevokeOrgMembership(context.Context, string, string) error
	ListOrgInvitations(context.Context, string, clerk.OrgInvitationListInput) ([]clerk.Invitation, error)
	GetOrgInvitation(context.Context, string, string) (clerk.Invitation, error)
	CreateOrgInvitation(context.Context, clerk.OrgInvitationInput) (clerk.Invitation, error)
	RevokeOrgInvitation(context.Context, string, string) error
}

// Processor is the topic-aware publisher for post-provisioning IAM commands.
// HTTP calls happen only after Prepare commits and before Finalize starts.
type Processor struct {
	clerk      clerkManager
	repository commandRepository
	now        func() time.Time
	locks      [64]sync.Mutex
}

func NewProcessor(provider clerkManager, repository commandRepository) (*Processor, error) {
	if provider == nil {
		return nil, errors.New("iam sync: Clerk manager is required")
	}
	if repository == nil {
		return nil, errors.New("iam sync: command repository is required")
	}
	return &Processor{
		clerk:      provider,
		repository: repository,
		now:        time.Now,
	}, nil
}

// Publish implements platform/outbox Publisher. Any provider or finalization
// failure is returned so the shared Dispatcher durably schedules a retry.
func (processor *Processor) Publish(
	ctx context.Context,
	publication platformoutbox.Publication,
) error {
	if processor == nil || processor.clerk == nil || processor.repository == nil {
		return errors.New("iam sync: processor is not configured")
	}
	event, err := decodeEvent(publication.Topic, publication.Payload)
	if err != nil {
		return err
	}
	if strings.TrimSpace(publication.MessageID) == "" ||
		!validIdempotencyKey(publication.IdempotencyKey, event) {
		return fmt.Errorf("%w: publication identity is invalid", ErrInvalidEvent)
	}

	lock := processor.organizationLock(event.OrganizationID)
	lock.Lock()
	defer lock.Unlock()

	prepared, err := processor.repository.Prepare(ctx, publication, event)
	if err != nil {
		return fmt.Errorf("prepare %s: %w", event.Operation, err)
	}
	result, err := processor.reconcile(ctx, prepared)
	if err != nil {
		return fmt.Errorf("reconcile %s with Clerk: %w", event.Operation, err)
	}
	if err := processor.repository.Finalize(ctx, publication, prepared, result); err != nil {
		return fmt.Errorf("finalize %s: %w", event.Operation, err)
	}
	return nil
}

func (processor *Processor) reconcile(ctx context.Context, prepared snapshot) (providerResult, error) {
	switch prepared.Action {
	case actionOrganizationUpdate:
		organization, err := processor.ensureOrganization(ctx, prepared)
		if err != nil {
			return providerResult{}, err
		}
		return providerResult{Organization: &organization}, nil
	case actionMembershipEnsure, actionOwnershipTransfer:
		membership, err := processor.ensureMembership(ctx, prepared)
		if err != nil {
			return providerResult{}, err
		}
		return providerResult{Membership: &membership}, nil
	case actionMembershipRemove:
		if err := processor.ensureMembershipRemoved(ctx, prepared); err != nil {
			return providerResult{}, err
		}
		return providerResult{MembershipGone: true}, nil
	case actionInvitationEnsure:
		invitation, err := processor.ensureInvitation(ctx, prepared)
		if err != nil {
			return providerResult{}, err
		}
		return providerResult{Invitation: &invitation}, nil
	case actionInvitationResend:
		invitation, err := processor.resendInvitation(ctx, prepared)
		if err != nil {
			return providerResult{}, err
		}
		return providerResult{Invitation: &invitation}, nil
	case actionInvitationRevoke:
		if err := processor.ensureInvitationRevoked(ctx, prepared); err != nil {
			return providerResult{}, err
		}
		return providerResult{InvitationGone: true}, nil
	case actionNoop:
		return providerResult{ProviderSkipped: true}, nil
	default:
		return providerResult{}, fmt.Errorf("%w: unsupported action %d", ErrDurableConflict, prepared.Action)
	}
}

func (processor *Processor) ensureOrganization(
	ctx context.Context,
	prepared snapshot,
) (clerk.Organization, error) {
	externalID := prepared.Organization.ExternalID
	organization, err := processor.clerk.GetOrganization(ctx, externalID)
	if err != nil {
		return clerk.Organization{}, err
	}
	if err := exactOrganization(organization, externalID); err != nil {
		return clerk.Organization{}, err
	}
	if strings.TrimSpace(organization.Name) == prepared.Organization.Name {
		return organization, nil
	}

	updated, updateErr := processor.clerk.UpdateOrganization(
		ctx,
		externalID,
		clerk.OrganizationInput{Name: prepared.Organization.Name},
	)
	if updateErr == nil {
		if err := exactOrganization(updated, externalID); err != nil {
			return clerk.Organization{}, err
		}
		if strings.TrimSpace(updated.Name) != prepared.Organization.Name {
			return clerk.Organization{}, fmt.Errorf("Clerk returned organization name %q", updated.Name)
		}
		return updated, nil
	}

	// Recover an update whose response was lost.
	reconciled, reconcileErr := processor.clerk.GetOrganization(ctx, externalID)
	if reconcileErr == nil &&
		exactOrganization(reconciled, externalID) == nil &&
		strings.TrimSpace(reconciled.Name) == prepared.Organization.Name {
		return reconciled, nil
	}
	return clerk.Organization{}, updateErr
}

func (processor *Processor) ensureMembership(
	ctx context.Context,
	prepared snapshot,
) (clerk.OrganizationMembership, error) {
	organizationID := prepared.Organization.ExternalID
	userID := prepared.MemberUser.ExternalID
	membership, found, err := processor.clerk.GetOrgMembership(ctx, organizationID, userID)
	if err != nil {
		return clerk.OrganizationMembership{}, err
	}
	if found {
		if err := exactMembership(membership, organizationID, userID); err != nil {
			return clerk.OrganizationMembership{}, err
		}
		if membership.Role == prepared.ProviderRole {
			return membership, nil
		}
	}

	input := clerk.OrgMembershipInput{
		ProviderOrgID:  organizationID,
		ProviderUserID: userID,
		Role:           prepared.ProviderRole,
	}
	var mutationErr error
	if found {
		mutationErr = processor.clerk.UpdateOrgMembership(ctx, input)
	} else {
		mutationErr = processor.clerk.CreateOrgMembership(ctx, input)
	}
	reconciled, reconciledFound, reconcileErr := processor.clerk.GetOrgMembership(
		ctx,
		organizationID,
		userID,
	)
	if reconcileErr == nil && reconciledFound {
		if err := exactMembership(reconciled, organizationID, userID); err != nil {
			return clerk.OrganizationMembership{}, err
		}
		if reconciled.Role == prepared.ProviderRole {
			return reconciled, nil
		}
	}
	if mutationErr != nil {
		return clerk.OrganizationMembership{}, mutationErr
	}
	if reconcileErr != nil {
		return clerk.OrganizationMembership{}, reconcileErr
	}
	return clerk.OrganizationMembership{}, fmt.Errorf(
		"Clerk membership role is not %q after reconciliation",
		prepared.ProviderRole,
	)
}

func (processor *Processor) ensureMembershipRemoved(
	ctx context.Context,
	prepared snapshot,
) error {
	organizationID := prepared.Organization.ExternalID
	userID := prepared.MemberUser.ExternalID
	_, found, err := processor.clerk.GetOrgMembership(ctx, organizationID, userID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	revokeErr := processor.clerk.RevokeOrgMembership(ctx, organizationID, userID)
	_, stillPresent, reconcileErr := processor.clerk.GetOrgMembership(ctx, organizationID, userID)
	if reconcileErr == nil && !stillPresent {
		return nil
	}
	if revokeErr != nil {
		return revokeErr
	}
	if reconcileErr != nil {
		return reconcileErr
	}
	return errors.New("Clerk membership still exists after revocation")
}

func (processor *Processor) ensureInvitation(
	ctx context.Context,
	prepared snapshot,
) (clerk.Invitation, error) {
	organizationID := prepared.Organization.ExternalID
	local := prepared.Invitation
	role := mustProviderRole(local.Role)
	if local.ExternalID != "" {
		existing, err := processor.clerk.GetOrgInvitation(ctx, organizationID, local.ExternalID)
		if err == nil {
			if err := exactInvitation(existing, organizationID, local.Email, role); err != nil {
				return clerk.Invitation{}, err
			}
			if existing.Status == "pending" {
				return existing, nil
			}
			return clerk.Invitation{}, fmt.Errorf(
				"Clerk invitation %q has terminal status %q",
				existing.ID,
				existing.Status,
			)
		} else if !clerk.IsNotFound(err) {
			return clerk.Invitation{}, err
		}
	}

	existing, found, err := processor.findPendingInvitation(ctx, organizationID, local.Email, role, "")
	if err != nil {
		return clerk.Invitation{}, err
	}
	if found {
		if local.ExternalID != "" && local.ExternalID != existing.ID {
			return clerk.Invitation{}, fmt.Errorf(
				"local invitation is bound to %q but Clerk has pending %q",
				local.ExternalID,
				existing.ID,
			)
		}
		return existing, nil
	}
	return processor.createInvitation(ctx, prepared)
}

func (processor *Processor) resendInvitation(
	ctx context.Context,
	prepared snapshot,
) (clerk.Invitation, error) {
	organizationID := prepared.Organization.ExternalID
	local := prepared.Invitation
	role := mustProviderRole(local.Role)
	oldID := local.ExternalID
	if oldID != "" {
		existing, err := processor.clerk.GetOrgInvitation(ctx, organizationID, oldID)
		if err == nil {
			if err := exactInvitation(existing, organizationID, local.Email, role); err != nil {
				return clerk.Invitation{}, err
			}
			if existing.Status == "pending" &&
				!existing.CreatedAt.IsZero() &&
				!existing.CreatedAt.Before(prepared.CommandCreated.Add(-time.Second)) {
				// The provider invitation was created after this resend command.
				// This covers both a lost finalization acknowledgement and an
				// earlier create command that delivered the requested email late.
				return existing, nil
			}
			if existing.Status == "pending" {
				if err := processor.clerk.RevokeOrgInvitation(ctx, organizationID, oldID); err != nil {
					return clerk.Invitation{}, err
				}
			} else if existing.Status == "accepted" {
				return clerk.Invitation{}, errors.New("Clerk invitation was already accepted")
			}
		} else if !clerk.IsNotFound(err) {
			return clerk.Invitation{}, err
		}
	}

	existing, found, err := processor.findPendingInvitation(
		ctx,
		organizationID,
		local.Email,
		role,
		oldID,
	)
	if err != nil {
		return clerk.Invitation{}, err
	}
	if found {
		return existing, nil
	}
	return processor.createInvitation(ctx, prepared)
}

func (processor *Processor) createInvitation(
	ctx context.Context,
	prepared snapshot,
) (clerk.Invitation, error) {
	local := prepared.Invitation
	organizationID := prepared.Organization.ExternalID
	role := mustProviderRole(local.Role)
	days := processor.invitationExpiryDays(local.ExpiresAt)
	created, createErr := processor.clerk.CreateOrgInvitation(ctx, clerk.OrgInvitationInput{
		ProviderOrgID:         organizationID,
		Email:                 local.Email,
		Role:                  role,
		InviterProviderUserID: prepared.ActorUser.ExternalID,
		ExpiresInDays:         days,
	})
	if createErr == nil {
		if err := exactPendingInvitation(created, organizationID, local.Email, role); err != nil {
			return clerk.Invitation{}, err
		}
		return created, nil
	}

	// Recover a create whose provider response was lost.
	reconciled, found, reconcileErr := processor.findPendingInvitation(
		ctx,
		organizationID,
		local.Email,
		role,
		"",
	)
	if reconcileErr == nil && found {
		return reconciled, nil
	}
	return clerk.Invitation{}, createErr
}

func (processor *Processor) ensureInvitationRevoked(
	ctx context.Context,
	prepared snapshot,
) error {
	organizationID := prepared.Organization.ExternalID
	local := prepared.Invitation
	role := mustProviderRole(local.Role)
	if local.ExternalID != "" {
		existing, err := processor.clerk.GetOrgInvitation(ctx, organizationID, local.ExternalID)
		if err == nil {
			if err := exactInvitation(existing, organizationID, local.Email, role); err != nil {
				return err
			}
			if existing.Status != "pending" {
				return nil
			}
			revokeErr := processor.clerk.RevokeOrgInvitation(ctx, organizationID, existing.ID)
			if revokeErr != nil {
				return revokeErr
			}
			return processor.confirmInvitationNotPending(ctx, organizationID, existing.ID)
		}
		if !clerk.IsNotFound(err) {
			return err
		}
	}

	existing, found, err := processor.findPendingInvitation(
		ctx,
		organizationID,
		local.Email,
		role,
		"",
	)
	if err != nil || !found {
		return err
	}
	if err := processor.clerk.RevokeOrgInvitation(ctx, organizationID, existing.ID); err != nil {
		return err
	}
	return processor.confirmInvitationNotPending(ctx, organizationID, existing.ID)
}

func (processor *Processor) confirmInvitationNotPending(
	ctx context.Context,
	organizationID string,
	invitationID string,
) error {
	invitation, err := processor.clerk.GetOrgInvitation(ctx, organizationID, invitationID)
	if clerk.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if invitation.Status == "pending" {
		return errors.New("Clerk invitation is still pending after revocation")
	}
	return nil
}

func (processor *Processor) findPendingInvitation(
	ctx context.Context,
	organizationID string,
	email string,
	role string,
	excludeID string,
) (clerk.Invitation, bool, error) {
	invitations, err := processor.clerk.ListOrgInvitations(
		ctx,
		organizationID,
		clerk.OrgInvitationListInput{
			ListInput: clerk.ListInput{Limit: 100},
			Statuses:  []string{"pending"},
			Email:     email,
		},
	)
	if err != nil {
		return clerk.Invitation{}, false, err
	}
	var match clerk.Invitation
	found := false
	for _, invitation := range invitations {
		if invitation.ID == excludeID ||
			strings.ToLower(strings.TrimSpace(invitation.Email)) != email ||
			strings.TrimSpace(invitation.Status) != "pending" {
			continue
		}
		if strings.TrimSpace(invitation.Role) != role {
			return clerk.Invitation{}, false, fmt.Errorf(
				"pending Clerk invitation for %q has role %q",
				email,
				invitation.Role,
			)
		}
		if found && match.ID != invitation.ID {
			return clerk.Invitation{}, false, fmt.Errorf(
				"multiple pending Clerk invitations exist for %q",
				email,
			)
		}
		match = invitation
		found = true
	}
	if !found {
		return clerk.Invitation{}, false, nil
	}
	if err := exactPendingInvitation(match, organizationID, email, role); err != nil {
		return clerk.Invitation{}, false, err
	}
	return match, true, nil
}

func (processor *Processor) invitationExpiryDays(expiry time.Time) int {
	now := processor.now()
	if now.IsZero() {
		now = time.Now()
	}
	days := int(math.Ceil(expiry.Sub(now.UTC()).Hours() / 24))
	if days < 1 {
		return 1
	}
	return days
}

func (processor *Processor) organizationLock(organizationID string) *sync.Mutex {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(organizationID))
	return &processor.locks[hash.Sum32()%uint32(len(processor.locks))]
}

func exactOrganization(organization clerk.Organization, expectedID string) error {
	if strings.TrimSpace(organization.ID) != expectedID {
		return fmt.Errorf("Clerk organization ID %q does not match %q", organization.ID, expectedID)
	}
	return nil
}

func exactMembership(
	membership clerk.OrganizationMembership,
	organizationID string,
	userID string,
) error {
	if strings.TrimSpace(membership.ID) == "" ||
		strings.TrimSpace(membership.OrganizationID) != organizationID ||
		strings.TrimSpace(membership.User.ID) != userID {
		return errors.New("Clerk membership does not match organization/user")
	}
	return nil
}

func exactInvitation(
	invitation clerk.Invitation,
	organizationID string,
	email string,
	role string,
) error {
	if strings.TrimSpace(invitation.ID) == "" ||
		strings.TrimSpace(invitation.OrganizationID) != organizationID ||
		strings.ToLower(strings.TrimSpace(invitation.Email)) != email ||
		strings.TrimSpace(invitation.Role) != role {
		return errors.New("Clerk invitation does not match organization/email/role")
	}
	return nil
}

func exactPendingInvitation(
	invitation clerk.Invitation,
	organizationID string,
	email string,
	role string,
) error {
	if err := exactInvitation(invitation, organizationID, email, role); err != nil {
		return err
	}
	if strings.TrimSpace(invitation.Status) != "pending" {
		return fmt.Errorf("Clerk invitation status is %q", invitation.Status)
	}
	return nil
}
