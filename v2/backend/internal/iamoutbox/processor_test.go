package iamoutbox

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
)

func TestProcessorCreatesProviderResourcesThenFinalizes(t *testing.T) {
	event := validProvisionEvent()
	expiry := time.Now().UTC().Add(7 * 24 * time.Hour)
	provider := &fakeClerkProvisioner{
		createdOrganization: clerk.Organization{
			ID: "org_created", Name: event.Name, Slug: event.Slug,
		},
		createdInvitation: clerk.Invitation{
			ID:             "orginv_created",
			OrganizationID: "org_created",
			Email:          event.OwnerEmail,
			Role:           event.ProviderRole,
			Status:         "pending",
			ExpiresAt:      &expiry,
		},
	}
	completion := &recordingFinalizer{}
	processor, err := NewProcessor(provider, completion)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	publication := publicationFor(t, event)
	if err := processor.Publish(context.Background(), publication); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if provider.createOrganizationCalls != 1 || provider.createInvitationCalls != 1 {
		t.Fatalf(
			"create calls = organization:%d invitation:%d",
			provider.createOrganizationCalls,
			provider.createInvitationCalls,
		)
	}
	if len(completion.results) != 1 {
		t.Fatalf("finalizations = %d, want 1", len(completion.results))
	}
	got := completion.results[0]
	if got.MessageID != publication.MessageID ||
		got.ProviderOrganization.ID != "org_created" ||
		got.ProviderInvitation.ID != "orginv_created" {
		t.Fatalf("finalization = %#v", got)
	}
}

func TestProcessorReusesExistingProviderResources(t *testing.T) {
	event := validProvisionEvent()
	expiry := time.Now().UTC().Add(3 * 24 * time.Hour)
	provider := &fakeClerkProvisioner{
		organizations: []clerk.Organization{
			{ID: "org_existing", Name: event.Name, Slug: event.Slug},
		},
		invitations: []clerk.Invitation{
			{
				ID:             "orginv_existing",
				OrganizationID: "org_existing",
				Email:          event.OwnerEmail,
				Role:           event.ProviderRole,
				Status:         "pending",
				ExpiresAt:      &expiry,
			},
		},
	}
	completion := &recordingFinalizer{}
	processor, err := NewProcessor(provider, completion)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	if err := processor.Publish(context.Background(), publicationFor(t, event)); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if provider.createOrganizationCalls != 0 ||
		provider.createInvitationCalls != 0 ||
		provider.updateOrganizationCalls != 0 {
		t.Fatalf("unexpected provider mutation calls: %#v", provider)
	}
	if len(completion.results) != 1 {
		t.Fatalf("finalizations = %d, want 1", len(completion.results))
	}
}

func TestProcessorReconcilesOrganizationDisplayName(t *testing.T) {
	event := validProvisionEvent()
	expiry := time.Now().UTC().Add(24 * time.Hour)
	provider := &fakeClerkProvisioner{
		organizations: []clerk.Organization{
			{ID: "org_existing", Name: "Old name", Slug: event.Slug},
		},
		updatedOrganization: clerk.Organization{
			ID: "org_existing", Name: event.Name, Slug: event.Slug,
		},
		invitations: []clerk.Invitation{
			{
				ID:             "orginv_existing",
				OrganizationID: "org_existing",
				Email:          event.OwnerEmail,
				Role:           event.ProviderRole,
				Status:         "pending",
				ExpiresAt:      &expiry,
			},
		},
	}
	processor, err := NewProcessor(provider, &recordingFinalizer{})
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}
	if err := processor.Publish(context.Background(), publicationFor(t, event)); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if provider.updateOrganizationCalls != 1 {
		t.Fatalf("UpdateOrganization calls = %d, want 1", provider.updateOrganizationCalls)
	}
}

func TestProcessorRecoversLostCreateResponseByNaturalKeys(t *testing.T) {
	event := validProvisionEvent()
	expiry := time.Now().UTC().Add(24 * time.Hour)
	provider := &fakeClerkProvisioner{
		listOrganizationsResults: [][]clerk.Organization{
			nil,
			{{ID: "org_race", Name: event.Name, Slug: event.Slug}},
		},
		createOrganizationErr: errors.New("connection reset after write"),
		listInvitationsResults: [][]clerk.Invitation{
			nil,
			{{
				ID:             "orginv_race",
				OrganizationID: "org_race",
				Email:          event.OwnerEmail,
				Role:           event.ProviderRole,
				Status:         "pending",
				ExpiresAt:      &expiry,
			}},
		},
		createInvitationErr: errors.New("connection reset after write"),
	}
	completion := &recordingFinalizer{}
	processor, err := NewProcessor(provider, completion)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}
	if err := processor.Publish(context.Background(), publicationFor(t, event)); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if len(completion.results) != 1 ||
		completion.results[0].ProviderOrganization.ID != "org_race" ||
		completion.results[0].ProviderInvitation.ID != "orginv_race" {
		t.Fatalf("finalizations = %#v", completion.results)
	}
}

func TestProcessorFailsClosedBeforeFinalization(t *testing.T) {
	event := validProvisionEvent()
	providerFailure := errors.New("Clerk unavailable")
	provider := &fakeClerkProvisioner{listOrganizationsErr: providerFailure}
	completion := &recordingFinalizer{}
	processor, err := NewProcessor(provider, completion)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	err = processor.Publish(context.Background(), publicationFor(t, event))
	if !errors.Is(err, providerFailure) {
		t.Fatalf("Publish() error = %v, want provider failure", err)
	}
	if len(completion.results) != 0 {
		t.Fatalf("finalizer called after provider failure: %#v", completion.results)
	}
}

func TestProcessorRejectsUnsupportedTopicAndMismatchedIdempotencyKey(t *testing.T) {
	processor, err := NewProcessor(&fakeClerkProvisioner{}, &recordingFinalizer{})
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}
	publication := publicationFor(t, validProvisionEvent())
	publication.Topic = "iam.invitation.create.requested.v1"
	if err := processor.Publish(context.Background(), publication); !errors.Is(err, ErrUnsupportedTopic) {
		t.Fatalf("unsupported topic error = %v", err)
	}

	publication = publicationFor(t, validProvisionEvent())
	publication.IdempotencyKey = "wrong"
	if err := processor.Publish(context.Background(), publication); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("idempotency mismatch error = %v", err)
	}
}

func TestDurableRequestMustMatchEveryEventIdentity(t *testing.T) {
	event := validProvisionEvent()
	request := durableProvisioningRequest{
		ID:              event.RequestID,
		OrganizationID:  event.OrganizationID,
		Provider:        event.Provider,
		Slug:            event.Slug,
		Name:            event.Name,
		OwnerEmail:      event.OwnerEmail,
		OutboxMessageID: "message-1",
		Status:          "queued",
	}
	result := Finalization{MessageID: "message-1", Event: event}
	if err := request.matches(result); err != nil {
		t.Fatalf("matches() error = %v", err)
	}

	changed := request
	changed.OwnerEmail = "attacker@example.test"
	if err := changed.matches(result); err == nil {
		t.Fatal("matches() accepted a changed durable owner")
	}
	result.MessageID = "message-2"
	if err := request.matches(result); err == nil {
		t.Fatal("matches() accepted a different outbox message")
	}
}

func publicationFor(t *testing.T, event ProvisionOrganizationEvent) platformoutbox.Publication {
	t.Helper()
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return platformoutbox.Publication{
		MessageID:      "message-1",
		IdempotencyKey: "iam.provision-org:" + event.Slug,
		Topic:          ProvisionOrganizationTopic,
		Payload:        payload,
		Headers:        map[string]string{"content-type": "application/json"},
		Attempt:        1,
		CreatedAt:      time.Now().UTC(),
	}
}

type fakeClerkProvisioner struct {
	organizations            []clerk.Organization
	listOrganizationsResults [][]clerk.Organization
	listOrganizationsErr     error
	createdOrganization      clerk.Organization
	createOrganizationErr    error
	updatedOrganization      clerk.Organization
	updateOrganizationErr    error
	invitations              []clerk.Invitation
	listInvitationsResults   [][]clerk.Invitation
	listInvitationsErr       error
	createdInvitation        clerk.Invitation
	createInvitationErr      error
	createOrganizationCalls  int
	updateOrganizationCalls  int
	createInvitationCalls    int
	listOrganizationsCalls   int
	listInvitationsCalls     int
}

func (provider *fakeClerkProvisioner) ListOrganizations(
	context.Context,
	clerk.ListInput,
) ([]clerk.Organization, error) {
	provider.listOrganizationsCalls++
	if provider.listOrganizationsErr != nil {
		return nil, provider.listOrganizationsErr
	}
	if len(provider.listOrganizationsResults) > 0 {
		index := min(provider.listOrganizationsCalls-1, len(provider.listOrganizationsResults)-1)
		return append([]clerk.Organization(nil), provider.listOrganizationsResults[index]...), nil
	}
	return append([]clerk.Organization(nil), provider.organizations...), nil
}

func (provider *fakeClerkProvisioner) CreateOrganization(
	_ context.Context,
	input clerk.OrganizationInput,
) (clerk.Organization, error) {
	provider.createOrganizationCalls++
	if provider.createOrganizationErr != nil {
		return clerk.Organization{}, provider.createOrganizationErr
	}
	if provider.createdOrganization.ID == "" {
		return clerk.Organization{ID: "org_created", Name: input.Name, Slug: input.Slug}, nil
	}
	return provider.createdOrganization, nil
}

func (provider *fakeClerkProvisioner) UpdateOrganization(
	_ context.Context,
	_ string,
	input clerk.OrganizationInput,
) (clerk.Organization, error) {
	provider.updateOrganizationCalls++
	if provider.updateOrganizationErr != nil {
		return clerk.Organization{}, provider.updateOrganizationErr
	}
	if provider.updatedOrganization.ID == "" {
		return clerk.Organization{ID: "org_existing", Name: input.Name, Slug: input.Slug}, nil
	}
	return provider.updatedOrganization, nil
}

func (provider *fakeClerkProvisioner) ListOrgInvitations(
	_ context.Context,
	_ string,
	_ clerk.OrgInvitationListInput,
) ([]clerk.Invitation, error) {
	provider.listInvitationsCalls++
	if provider.listInvitationsErr != nil {
		return nil, provider.listInvitationsErr
	}
	if len(provider.listInvitationsResults) > 0 {
		index := min(provider.listInvitationsCalls-1, len(provider.listInvitationsResults)-1)
		return append([]clerk.Invitation(nil), provider.listInvitationsResults[index]...), nil
	}
	return append([]clerk.Invitation(nil), provider.invitations...), nil
}

func (provider *fakeClerkProvisioner) CreateOrgInvitation(
	_ context.Context,
	input clerk.OrgInvitationInput,
) (clerk.Invitation, error) {
	provider.createInvitationCalls++
	if provider.createInvitationErr != nil {
		return clerk.Invitation{}, provider.createInvitationErr
	}
	if provider.createdInvitation.ID == "" {
		expiry := time.Now().UTC().Add(7 * 24 * time.Hour)
		return clerk.Invitation{
			ID:             "orginv_created",
			OrganizationID: input.ProviderOrgID,
			Email:          input.Email,
			Role:           input.Role,
			Status:         "pending",
			ExpiresAt:      &expiry,
		}, nil
	}
	return provider.createdInvitation, nil
}

type recordingFinalizer struct {
	results []Finalization
	err     error
}

func (completion *recordingFinalizer) Finalize(
	_ context.Context,
	result Finalization,
) error {
	completion.results = append(completion.results, result)
	return completion.err
}

func TestFinalizationFixtureDoesNotExposeSecrets(t *testing.T) {
	resultType := reflect.TypeFor[Finalization]()
	for index := range resultType.NumField() {
		name := resultType.Field(index).Name
		if name == "Ticket" || name == "Token" || name == "Secret" {
			t.Fatalf("Finalization unexpectedly exposes %s", name)
		}
	}
}
