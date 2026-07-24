package iamsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	platformiam "github.com/devpablocristo/platform/iam/go"
	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
)

func TestProcessorReplaysOrganizationAndMembershipCommandsWithoutDuplicateEffects(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 7, 23, 18, 0, 0, 0, time.UTC)

	t.Run("organization", func(t *testing.T) {
		event := eventFixture(organizationUpdateOperation, func(event *Event) {
			event.ResourceID = event.OrganizationID
			event.Name = "Acme nueva"
			event.AppliedLocally = true
		})
		prepared := baseSnapshot(event, actionOrganizationUpdate, now)
		prepared.Organization.Name = event.Name
		provider := newFakeClerk(now)
		provider.organization.Name = "Acme anterior"
		repository := &fakeRepository{prepared: prepared}
		processor := mustProcessor(t, provider, repository, now)
		publication := publicationFixture(t, OrganizationUpdateTopic, event, now)

		if err := processor.Publish(ctx, publication); err != nil {
			t.Fatalf("first Publish() error = %v", err)
		}
		if err := processor.Publish(ctx, publication); err != nil {
			t.Fatalf("replayed Publish() error = %v", err)
		}
		if provider.updateOrganizationCalls != 1 {
			t.Fatalf("UpdateOrganization calls = %d, want 1", provider.updateOrganizationCalls)
		}
		if repository.finalizeCalls != 2 {
			t.Fatalf("Finalize calls = %d, want 2", repository.finalizeCalls)
		}
	})

	t.Run("role reduction", func(t *testing.T) {
		event := eventFixture(memberRoleChangeOperation, func(event *Event) {
			event.Role = "member"
			event.PreviousRole = "admin"
			event.AppliedLocally = true
		})
		prepared := baseSnapshot(event, actionMembershipEnsure, now)
		prepared.ProviderRole = clerkMemberRole
		prepared.Membership = membershipFixture(event.ResourceID, event.OrganizationID, "member")
		prepared.MemberUser = userFixture("user_target")
		provider := newFakeClerk(now)
		provider.membership = membershipProviderFixture("org:admin")
		repository := &fakeRepository{prepared: prepared}
		processor := mustProcessor(t, provider, repository, now)
		publication := publicationFixture(t, MemberRoleChangeTopic, event, now)

		if err := processor.Publish(ctx, publication); err != nil {
			t.Fatalf("first Publish() error = %v", err)
		}
		if err := processor.Publish(ctx, publication); err != nil {
			t.Fatalf("replayed Publish() error = %v", err)
		}
		if provider.updateMembershipCalls != 1 {
			t.Fatalf("UpdateOrgMembership calls = %d, want 1", provider.updateMembershipCalls)
		}
		if provider.membership.Role != clerkMemberRole {
			t.Fatalf("provider role = %q", provider.membership.Role)
		}
	})

	t.Run("removal", func(t *testing.T) {
		event := eventFixture(memberRemoveOperation, func(event *Event) {
			event.PreviousRole = "member"
			event.AppliedLocally = true
		})
		prepared := baseSnapshot(event, actionMembershipRemove, now)
		prepared.Membership = membershipFixture(event.ResourceID, event.OrganizationID, "member")
		prepared.Membership.Status = platformiam.MembershipRemoved
		prepared.MemberUser = userFixture("user_target")
		provider := newFakeClerk(now)
		provider.membership = membershipProviderFixture(clerkMemberRole)
		repository := &fakeRepository{prepared: prepared}
		processor := mustProcessor(t, provider, repository, now)
		publication := publicationFixture(t, MemberRemoveTopic, event, now)

		if err := processor.Publish(ctx, publication); err != nil {
			t.Fatalf("first Publish() error = %v", err)
		}
		if err := processor.Publish(ctx, publication); err != nil {
			t.Fatalf("replayed Publish() error = %v", err)
		}
		if provider.revokeMembershipCalls != 1 {
			t.Fatalf("RevokeOrgMembership calls = %d, want 1", provider.revokeMembershipCalls)
		}
		if !repository.lastResult.MembershipGone {
			t.Fatal("final result did not preserve local removal")
		}
	})
}

func TestProcessorRecoversInvitationCreateAndResendReplays(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 7, 23, 18, 0, 0, 0, time.UTC)
	expiry := now.Add(7 * 24 * time.Hour)

	t.Run("lost create response", func(t *testing.T) {
		event := invitationEventFixture(invitationCreateOperation, expiry, true)
		prepared := baseSnapshot(event, actionInvitationEnsure, now)
		prepared.Invitation = invitationFixture(event, "")
		provider := newFakeClerk(now)
		provider.failNextCreateAfterWrite = true
		repository := &fakeRepository{prepared: prepared, attachInvitation: true}
		processor := mustProcessor(t, provider, repository, now)
		publication := publicationFixture(t, InvitationCreateTopic, event, now)

		if err := processor.Publish(ctx, publication); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
		if provider.createInvitationCalls != 1 {
			t.Fatalf("CreateOrgInvitation calls = %d, want 1", provider.createInvitationCalls)
		}
		if repository.lastResult.Invitation == nil {
			t.Fatal("provider invitation was not finalized")
		}
	})

	t.Run("resend replay", func(t *testing.T) {
		event := invitationEventFixture(invitationResendOperation, expiry, false)
		event.ExternalResourceID = "inv_old"
		prepared := baseSnapshot(event, actionInvitationResend, now)
		prepared.Invitation = invitationFixture(event, "inv_old")
		provider := newFakeClerk(now)
		provider.invitations["inv_old"] = clerk.Invitation{
			ID:             "inv_old",
			OrganizationID: "org_acme",
			Email:          event.Email,
			Role:           clerkMemberRole,
			Status:         "pending",
			CreatedAt:      now.Add(-24 * time.Hour),
		}
		repository := &fakeRepository{prepared: prepared, attachInvitation: true}
		processor := mustProcessor(t, provider, repository, now)
		publication := publicationFixture(t, InvitationResendTopic, event, now)

		if err := processor.Publish(ctx, publication); err != nil {
			t.Fatalf("first Publish() error = %v", err)
		}
		if err := processor.Publish(ctx, publication); err != nil {
			t.Fatalf("replayed Publish() error = %v", err)
		}
		if provider.createInvitationCalls != 1 || provider.revokeInvitationCalls != 1 {
			t.Fatalf(
				"provider calls = create:%d revoke:%d, want 1/1",
				provider.createInvitationCalls,
				provider.revokeInvitationCalls,
			)
		}
	})
}

func TestProcessorMakesOwnershipProviderFirstAndRevokesInvitations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 7, 23, 18, 0, 0, 0, time.UTC)

	t.Run("ownership", func(t *testing.T) {
		event := eventFixture(ownershipTransferOperation, func(event *Event) {
			event.Role = "owner"
			event.PreviousRole = "admin"
			event.ExternalResourceID = "membership_target"
			event.AppliedLocally = false
		})
		prepared := baseSnapshot(event, actionOwnershipTransfer, now)
		prepared.ProviderRole = clerkAdministratorRole
		prepared.ApplyOwnership = true
		prepared.Membership = membershipFixture(event.ResourceID, event.OrganizationID, "admin")
		prepared.MemberUser = userFixture("user_target")
		provider := newFakeClerk(now)
		provider.membership = membershipProviderFixture(clerkMemberRole)
		repository := &fakeRepository{prepared: prepared}
		processor := mustProcessor(t, provider, repository, now)
		publication := publicationFixture(t, OwnershipTransferTopic, event, now)

		if err := processor.Publish(ctx, publication); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
		if provider.updateMembershipCalls != 1 {
			t.Fatalf("UpdateOrgMembership calls = %d, want 1", provider.updateMembershipCalls)
		}
		if repository.lastResult.Membership == nil ||
			repository.lastResult.Membership.Role != clerkAdministratorRole {
			t.Fatal("ownership was finalized without verified Clerk admin role")
		}
	})

	t.Run("invitation revoke", func(t *testing.T) {
		expiry := now.Add(7 * 24 * time.Hour)
		event := invitationEventFixture(invitationRevokeOperation, expiry, true)
		event.ExternalResourceID = "inv_revoke"
		prepared := baseSnapshot(event, actionInvitationRevoke, now)
		prepared.Invitation = invitationFixture(event, "inv_revoke")
		prepared.Invitation.Status = platformiam.InvitationRevoked
		provider := newFakeClerk(now)
		provider.invitations["inv_revoke"] = clerk.Invitation{
			ID:             "inv_revoke",
			OrganizationID: "org_acme",
			Email:          event.Email,
			Role:           clerkMemberRole,
			Status:         "pending",
			CreatedAt:      now.Add(-time.Hour),
		}
		repository := &fakeRepository{prepared: prepared}
		processor := mustProcessor(t, provider, repository, now)
		publication := publicationFixture(t, InvitationRevokeTopic, event, now)

		if err := processor.Publish(ctx, publication); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
		if provider.revokeInvitationCalls != 1 || !repository.lastResult.InvitationGone {
			t.Fatalf(
				"revocation = calls:%d gone:%v",
				provider.revokeInvitationCalls,
				repository.lastResult.InvitationGone,
			)
		}
	})
}

func TestProcessorSerializesConcurrentInvitationReplay(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 23, 18, 0, 0, 0, time.UTC)
	expiry := now.Add(7 * 24 * time.Hour)
	event := invitationEventFixture(invitationCreateOperation, expiry, true)
	prepared := baseSnapshot(event, actionInvitationEnsure, now)
	prepared.Invitation = invitationFixture(event, "")
	provider := newFakeClerk(now)
	repository := &fakeRepository{prepared: prepared, attachInvitation: true}
	processor := mustProcessor(t, provider, repository, now)
	publication := publicationFixture(t, InvitationCreateTopic, event, now)

	const workers = 24
	var (
		waitGroup sync.WaitGroup
		failures  = make(chan error, workers)
	)
	for range workers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			if err := processor.Publish(context.Background(), publication); err != nil {
				failures <- err
			}
		}()
	}
	waitGroup.Wait()
	close(failures)
	for err := range failures {
		t.Errorf("concurrent Publish() error = %v", err)
	}
	if provider.createInvitationCalls != 1 {
		t.Fatalf("CreateOrgInvitation calls = %d, want 1", provider.createInvitationCalls)
	}
	if repository.finalizeCalls != workers {
		t.Fatalf("Finalize calls = %d, want %d", repository.finalizeCalls, workers)
	}
}

type fakeRepository struct {
	mu               sync.Mutex
	prepared         snapshot
	finalizeCalls    int
	lastResult       providerResult
	attachInvitation bool
}

func (repository *fakeRepository) Prepare(
	_ context.Context,
	_ platformoutbox.Publication,
	_ Event,
) (snapshot, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.prepared, nil
}

func (repository *fakeRepository) Finalize(
	_ context.Context,
	_ platformoutbox.Publication,
	_ snapshot,
	result providerResult,
) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.finalizeCalls++
	repository.lastResult = result
	if repository.attachInvitation && result.Invitation != nil {
		repository.prepared.Invitation.ExternalID = result.Invitation.ID
	}
	return nil
}

type fakeClerk struct {
	mu                       sync.Mutex
	now                      time.Time
	organization             clerk.Organization
	membership               clerk.OrganizationMembership
	membershipFound          bool
	invitations              map[string]clerk.Invitation
	updateOrganizationCalls  int
	createMembershipCalls    int
	updateMembershipCalls    int
	revokeMembershipCalls    int
	createInvitationCalls    int
	revokeInvitationCalls    int
	failNextCreateAfterWrite bool
}

func newFakeClerk(now time.Time) *fakeClerk {
	return &fakeClerk{
		now: now,
		organization: clerk.Organization{
			ID:   "org_acme",
			Name: "Acme",
			Slug: "acme",
		},
		membershipFound: true,
		invitations:     make(map[string]clerk.Invitation),
	}
}

func (provider *fakeClerk) GetOrganization(
	_ context.Context,
	_ string,
) (clerk.Organization, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return provider.organization, nil
}

func (provider *fakeClerk) UpdateOrganization(
	_ context.Context,
	_ string,
	input clerk.OrganizationInput,
) (clerk.Organization, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.updateOrganizationCalls++
	provider.organization.Name = input.Name
	return provider.organization, nil
}

func (provider *fakeClerk) GetOrgMembership(
	_ context.Context,
	_ string,
	_ string,
) (clerk.OrganizationMembership, bool, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return provider.membership, provider.membershipFound, nil
}

func (provider *fakeClerk) CreateOrgMembership(
	_ context.Context,
	input clerk.OrgMembershipInput,
) error {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.createMembershipCalls++
	provider.membershipFound = true
	provider.membership = membershipProviderFixture(input.Role)
	return nil
}

func (provider *fakeClerk) UpdateOrgMembership(
	_ context.Context,
	input clerk.OrgMembershipInput,
) error {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.updateMembershipCalls++
	provider.membership.Role = input.Role
	return nil
}

func (provider *fakeClerk) RevokeOrgMembership(
	_ context.Context,
	_ string,
	_ string,
) error {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.revokeMembershipCalls++
	provider.membershipFound = false
	provider.membership = clerk.OrganizationMembership{}
	return nil
}

func (provider *fakeClerk) ListOrgInvitations(
	_ context.Context,
	_ string,
	input clerk.OrgInvitationListInput,
) ([]clerk.Invitation, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	result := make([]clerk.Invitation, 0)
	for _, invitation := range provider.invitations {
		if invitation.Status == "pending" && invitation.Email == input.Email {
			result = append(result, invitation)
		}
	}
	return result, nil
}

func (provider *fakeClerk) GetOrgInvitation(
	_ context.Context,
	_ string,
	invitationID string,
) (clerk.Invitation, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	invitation, ok := provider.invitations[invitationID]
	if !ok {
		return clerk.Invitation{}, &clerk.APIError{StatusCode: 404}
	}
	return invitation, nil
}

func (provider *fakeClerk) CreateOrgInvitation(
	_ context.Context,
	input clerk.OrgInvitationInput,
) (clerk.Invitation, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.createInvitationCalls++
	id := fmt.Sprintf("inv_%d", provider.createInvitationCalls)
	expiry := provider.now.Add(time.Duration(input.ExpiresInDays) * 24 * time.Hour)
	invitation := clerk.Invitation{
		ID:             id,
		OrganizationID: input.ProviderOrgID,
		Email:          input.Email,
		Role:           input.Role,
		Status:         "pending",
		ExpiresAt:      &expiry,
		CreatedAt:      provider.now.Add(time.Duration(provider.createInvitationCalls) * time.Second),
	}
	provider.invitations[id] = invitation
	if provider.failNextCreateAfterWrite {
		provider.failNextCreateAfterWrite = false
		return clerk.Invitation{}, errors.New("response lost")
	}
	return invitation, nil
}

func (provider *fakeClerk) RevokeOrgInvitation(
	_ context.Context,
	_ string,
	invitationID string,
) error {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.revokeInvitationCalls++
	invitation, ok := provider.invitations[invitationID]
	if !ok {
		return nil
	}
	invitation.Status = "revoked"
	provider.invitations[invitationID] = invitation
	return nil
}

func mustProcessor(
	t *testing.T,
	provider clerkManager,
	repository commandRepository,
	now time.Time,
) *Processor {
	t.Helper()
	processor, err := NewProcessor(provider, repository)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}
	processor.now = func() time.Time { return now }
	return processor
}

func publicationFixture(
	t *testing.T,
	topic string,
	event Event,
	createdAt time.Time,
) platformoutbox.Publication {
	t.Helper()
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return platformoutbox.Publication{
		MessageID:      "message-1",
		IdempotencyKey: "iam:" + event.OrganizationID + ":" + event.Operation + ":command1",
		Topic:          topic,
		Payload:        payload,
		CreatedAt:      createdAt,
	}
}

func baseSnapshot(event Event, action action, createdAt time.Time) snapshot {
	return snapshot{
		MessageID:      "message-1",
		CommandCreated: createdAt,
		Event:          event,
		Action:         action,
		Organization: platformiam.Organization{
			ID:         event.OrganizationID,
			Provider:   providerClerk,
			ExternalID: event.ExternalOrganizationID,
			Name:       "Acme",
			Slug:       "acme",
			Status:     platformiam.OrganizationActive,
		},
		Actor: membershipFixture(
			event.ActorMembershipID,
			event.OrganizationID,
			"owner",
		),
		ActorUser: userFixture("user_actor"),
	}
}

func membershipFixture(id, organizationID, role string) platformiam.Membership {
	return platformiam.Membership{
		ID:             id,
		OrganizationID: organizationID,
		UserID:         "50000000-0000-4000-8000-000000000001",
		Provider:       providerClerk,
		ExternalID:     "membership_target",
		Role:           role,
		Status:         platformiam.MembershipActive,
	}
}

func userFixture(externalID string) platformiam.User {
	return platformiam.User{
		ID:            "50000000-0000-4000-8000-000000000001",
		Provider:      providerClerk,
		ExternalID:    externalID,
		PrimaryEmail:  externalID + "@example.test",
		EmailVerified: true,
		Status:        platformiam.UserActive,
	}
}

func invitationFixture(event Event, externalID string) platformiam.Invitation {
	return platformiam.Invitation{
		ID:             event.ResourceID,
		OrganizationID: event.OrganizationID,
		Provider:       providerClerk,
		ExternalID:     externalID,
		Email:          event.Email,
		Role:           event.Role,
		Status:         platformiam.InvitationPending,
		ExpiresAt:      *event.ExpiresAt,
	}
}

func membershipProviderFixture(role string) clerk.OrganizationMembership {
	return clerk.OrganizationMembership{
		ID:             "membership_target",
		Role:           role,
		OrganizationID: "org_acme",
		User:           clerk.User{ID: "user_target"},
	}
}
