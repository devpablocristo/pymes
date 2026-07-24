package iamoutbox

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/provisioning"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresProvisioningFinalizationIsIdempotent(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	database, err := postgres.OpenWithConfig(
		ctx,
		databaseURL,
		postgres.DefaultConfig("pymes-v2-iam-outbox-finalizer-test"),
	)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer database.Close()
	workerDatabase := openIAMWorkerTestDatabase(t, ctx, databaseURL)
	defer workerDatabase.Close()

	provisioner, err := provisioning.NewService(database.Pool())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	slug := "worker-" + uuid.NewString()[:8]
	result, err := provisioner.Provision(ctx, provisioning.Input{
		Name:       "Worker Integration",
		Slug:       slug,
		OwnerEmail: "owner+" + slug + "@example.test",
	})
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}
	defer cleanupWorkerFixture(t, database, result)

	var payload []byte
	if err := database.QueryRow(ctx, `
		SELECT payload
		FROM public.platform_outbox_messages
		WHERE id = $1
	`, result.OutboxMessageID).Scan(&payload); err != nil {
		t.Fatalf("load outbox payload: %v", err)
	}
	event, err := decodeProvisionOrganizationEvent(payload)
	if err != nil {
		t.Fatalf("decode event: %v", err)
	}
	expiry := time.Now().UTC().Add(7 * 24 * time.Hour)
	organization := clerkOrganizationFixture(event)
	invitation := clerkInvitationFixture(event, organization.ID, expiry)
	provider := &fakeClerkProvisioner{
		listOrganizationsResults: [][]clerk.Organization{
			nil,
			{organization},
		},
		createdOrganization: organization,
		listInvitationsResults: [][]clerk.Invitation{
			nil,
			{invitation},
		},
		createdInvitation: invitation,
	}
	completion, err := NewPostgresFinalizer(workerDatabase.Pool())
	if err != nil {
		t.Fatalf("NewPostgresFinalizer() error = %v", err)
	}
	processor, err := NewProcessor(provider, completion)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}
	publication := platformoutbox.Publication{
		MessageID:      result.OutboxMessageID,
		IdempotencyKey: "iam.provision-org:" + result.Slug,
		Topic:          ProvisionOrganizationTopic,
		Payload:        payload,
	}
	if err := processor.Publish(ctx, publication); err != nil {
		t.Fatalf("first Publish() error = %v", err)
	}
	if err := processor.Publish(ctx, publication); err != nil {
		t.Fatalf("replayed Publish() error = %v", err)
	}

	var (
		requestStatus        string
		externalOrganization string
		invitationCount      int
		externalInvitation   string
		invitationRole       string
		invitationStatus     string
	)
	if err := database.QueryRow(ctx, `
		SELECT status
		FROM app.organization_provisioning_requests
		WHERE id = $1::uuid
	`, result.RequestID).Scan(&requestStatus); err != nil {
		t.Fatalf("load request status: %v", err)
	}
	if err := database.QueryRow(ctx, `
		SELECT external_id
		FROM iam.organizations
		WHERE id = $1::uuid
	`, result.OrganizationID).Scan(&externalOrganization); err != nil {
		t.Fatalf("load organization external ID: %v", err)
	}
	if err := database.QueryRow(ctx, `
		SELECT count(*), max(external_id), max(role), max(status)
		FROM iam.invitations
		WHERE org_id = $1::uuid
		  AND email_normalized = $2
	`, result.OrganizationID, result.OwnerEmail).Scan(
		&invitationCount,
		&externalInvitation,
		&invitationRole,
		&invitationStatus,
	); err != nil {
		t.Fatalf("load local invitation: %v", err)
	}
	if requestStatus != "provisioned" ||
		externalOrganization != organization.ID ||
		invitationCount != 1 ||
		externalInvitation != invitation.ID ||
		invitationRole != LocalOwnerRole ||
		invitationStatus != "pending" {
		t.Fatalf(
			"final state = request:%q org:%q invitations:%d/%q/%q/%q",
			requestStatus,
			externalOrganization,
			invitationCount,
			externalInvitation,
			invitationRole,
			invitationStatus,
		)
	}
}

func TestProvisioningOutboxViewDoesNotLeaseOtherIAMContracts(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	database, err := postgres.OpenWithConfig(
		ctx,
		databaseURL,
		postgres.DefaultConfig("pymes-v2-iam-topic-store-test"),
	)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer database.Close()
	workerDatabase := openIAMWorkerTestDatabase(t, ctx, databaseURL)
	defer workerDatabase.Close()

	baseStore, err := platformoutbox.NewStore(database.Pool(), platformoutbox.StoreConfig{
		Table:              pgx.Identifier{platformoutbox.DefaultTableName},
		DefaultMaxAttempts: 3,
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	targetID := uuid.NewString()
	otherID := uuid.NewString()
	defer func() {
		_, cleanupErr := database.Exec(
			context.Background(),
			"DELETE FROM public.platform_outbox_messages WHERE id = ANY($1::text[])",
			[]string{targetID, otherID},
		)
		if cleanupErr != nil {
			t.Errorf("cleanup outbox fixtures: %v", cleanupErr)
		}
	}()

	tx, err := database.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin append transaction: %v", err)
	}
	if _, err := baseStore.Append(ctx, tx, platformoutbox.MessageInput{
		ID: targetID, Topic: ProvisionOrganizationTopic, Payload: []byte(`{}`),
	}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("append target: %v", err)
	}
	if _, err := baseStore.Append(ctx, tx, platformoutbox.MessageInput{
		ID: otherID, Topic: "iam.invitation.create.requested.v1", Payload: []byte(`{}`),
	}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("append other topic: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit append transaction: %v", err)
	}

	store, err := platformoutbox.NewStore(workerDatabase.Pool(), platformoutbox.StoreConfig{
		Table:              provisioningOutboxView,
		DefaultMaxAttempts: 3,
	})
	if err != nil {
		t.Fatalf("NewStore(provisioning view) error = %v", err)
	}
	leased, err := store.Lease(ctx, platformoutbox.LeaseRequest{
		Limit: 10, Duration: time.Minute,
	})
	if err != nil {
		t.Fatalf("Lease() error = %v", err)
	}
	if len(leased) != 1 || leased[0].ID != targetID {
		t.Fatalf("leased = %#v, want only %q", leased, targetID)
	}
	if err := store.MarkPublished(ctx, leased[0]); err != nil {
		t.Fatalf("MarkPublished() error = %v", err)
	}

	var (
		otherAttempts  int
		otherPublished *time.Time
	)
	if err := database.QueryRow(ctx, `
		SELECT attempts, published_at
		FROM public.platform_outbox_messages
		WHERE id = $1
	`, otherID).Scan(&otherAttempts, &otherPublished); err != nil {
		t.Fatalf("load other topic state: %v", err)
	}
	if otherAttempts != 0 || otherPublished != nil {
		t.Fatalf("other topic state = attempts:%d published:%v", otherAttempts, otherPublished)
	}
}

func openIAMWorkerTestDatabase(
	t *testing.T,
	ctx context.Context,
	databaseURL string,
) *postgres.DB {
	t.Helper()
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse IAM worker test database URL: %v", err)
	}
	config.ConnConfig.User = "pymes_iam_worker"
	config.ConnConfig.Password = "pymes_iam_worker"
	database, err := postgres.OpenWithConfig(
		ctx,
		config.ConnString(),
		postgres.DefaultConfig("pymes-v2-iam-worker-role-test"),
	)
	if err != nil {
		t.Fatalf("open IAM worker database: %v", err)
	}
	return database
}

func cleanupWorkerFixture(t *testing.T, database *postgres.DB, result provisioning.Result) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	queries := []struct {
		sql  string
		args []any
	}{
		{
			sql:  "DELETE FROM public.platform_outbox_messages WHERE id = $1",
			args: []any{result.OutboxMessageID},
		},
		{
			sql:  "DELETE FROM app.organization_provisioning_requests WHERE id = $1::uuid",
			args: []any{result.RequestID},
		},
		{
			sql:  "DELETE FROM iam.invitations WHERE org_id = $1::uuid",
			args: []any{result.OrganizationID},
		},
		{
			sql:  "DELETE FROM iam.organizations WHERE id = $1::uuid",
			args: []any{result.OrganizationID},
		},
	}
	for _, query := range queries {
		if _, err := database.Exec(ctx, query.sql, query.args...); err != nil {
			t.Errorf("cleanup query %q: %v", query.sql, err)
		}
	}
}

func clerkOrganizationFixture(event ProvisionOrganizationEvent) clerk.Organization {
	return clerk.Organization{
		ID:   "org_" + strings.ReplaceAll(event.Slug, "-", "_"),
		Name: event.Name,
		Slug: event.Slug,
	}
}

func clerkInvitationFixture(
	event ProvisionOrganizationEvent,
	organizationID string,
	expiry time.Time,
) clerk.Invitation {
	return clerk.Invitation{
		ID:             "orginv_" + strings.ReplaceAll(event.Slug, "-", "_"),
		OrganizationID: organizationID,
		Email:          event.OwnerEmail,
		Role:           event.ProviderRole,
		Status:         "pending",
		ExpiresAt:      &expiry,
	}
}
