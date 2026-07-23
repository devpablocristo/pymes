package iamsync

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestPostgresOwnershipFinalizationIsAtomicAndReplaySafe(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	database, err := postgres.OpenWithConfig(
		ctx,
		databaseURL,
		postgres.DefaultConfig("pymes-v2-iam-sync-finalizer-test"),
	)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer database.Close()

	organizationID := uuid.NewString()
	actorUserID := uuid.NewString()
	targetUserID := uuid.NewString()
	actorMembershipID := uuid.NewString()
	targetMembershipID := uuid.NewString()
	messageID := uuid.NewString()
	externalOrganizationID := "org_" + uuid.NewString()
	targetProviderUserID := "user_" + uuid.NewString()
	targetProviderMembershipID := "orgmem_" + uuid.NewString()
	defer cleanupIAMSyncFixture(t, database, organizationID, messageID)

	tx, err := database.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin fixture: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO iam.organizations (id, provider, external_id, name, slug, status)
		VALUES ($1::uuid, 'clerk', $2, 'Ownership integration', $3, 'provisioning')
	`, organizationID, externalOrganizationID, "ownership-"+organizationID[:8]); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("insert organization: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO iam.users (
			id, provider, external_id, primary_email, email_verified, name, status
		) VALUES
			($1::uuid, 'clerk', $2, $3, true, 'Old owner', 'active'),
			($4::uuid, 'clerk', $5, $6, true, 'New owner', 'active')
	`, actorUserID, "user_"+actorUserID, actorUserID+"@example.test",
		targetUserID, targetProviderUserID, targetUserID+"@example.test"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("insert users: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO iam.memberships (
			id, org_id, user_id, provider, external_id, role, status, joined_at
		) VALUES
			($1::uuid, $2::uuid, $3::uuid, 'clerk', $4, 'owner', 'active', now()),
			($5::uuid, $2::uuid, $6::uuid, 'clerk', $7, 'admin', 'active', now())
	`, actorMembershipID, organizationID, actorUserID, "orgmem_"+actorMembershipID,
		targetMembershipID, targetUserID, targetProviderMembershipID); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("insert memberships: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE iam.organizations SET status = 'active' WHERE id = $1::uuid
	`, organizationID); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("activate organization: %v", err)
	}

	event := Event{
		SchemaVersion:          1,
		Operation:              ownershipTransferOperation,
		OrganizationID:         organizationID,
		ExternalOrganizationID: externalOrganizationID,
		ActorUserID:            actorUserID,
		ActorMembershipID:      actorMembershipID,
		ResourceID:             targetMembershipID,
		ExternalResourceID:     targetProviderMembershipID,
		Role:                   "owner",
		PreviousRole:           "admin",
		AppliedLocally:         false,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("marshal event: %v", err)
	}
	outbox, err := platformoutbox.NewStore(database.Pool(), platformoutbox.StoreConfig{
		Table:              pgx.Identifier{"public", platformoutbox.DefaultTableName},
		DefaultMaxAttempts: 12,
	})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("create outbox store: %v", err)
	}
	idempotencyKey := "iam:" + organizationID + ":" + ownershipTransferOperation + ":integration"
	if _, err := outbox.Append(ctx, tx, platformoutbox.MessageInput{
		ID:             messageID,
		IdempotencyKey: idempotencyKey,
		Topic:          OwnershipTransferTopic,
		Payload:        payload,
	}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("append outbox event: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit fixture: %v", err)
	}

	repository, err := NewPostgresRepository(database.Pool())
	if err != nil {
		t.Fatalf("NewPostgresRepository() error = %v", err)
	}
	publication := platformoutbox.Publication{
		MessageID:      messageID,
		IdempotencyKey: idempotencyKey,
		Topic:          OwnershipTransferTopic,
		Payload:        payload,
	}
	prepared, err := repository.Prepare(ctx, publication, event)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if !prepared.ApplyOwnership || prepared.ProviderRole != clerkAdministratorRole {
		t.Fatalf("prepared = %#v", prepared)
	}
	result := providerResult{Membership: &clerk.OrganizationMembership{
		ID:             targetProviderMembershipID,
		Role:           clerkAdministratorRole,
		OrganizationID: externalOrganizationID,
		User:           clerk.User{ID: targetProviderUserID},
	}}
	if err := repository.Finalize(ctx, publication, prepared, result); err != nil {
		t.Fatalf("first Finalize() error = %v", err)
	}
	if err := repository.Finalize(ctx, publication, prepared, result); err != nil {
		t.Fatalf("replayed Finalize() error = %v", err)
	}

	var actorRole, targetRole string
	if err := database.QueryRow(ctx, `
		SELECT
			(SELECT role FROM iam.memberships WHERE id = $1::uuid),
			(SELECT role FROM iam.memberships WHERE id = $2::uuid)
	`, actorMembershipID, targetMembershipID).Scan(&actorRole, &targetRole); err != nil {
		t.Fatalf("load final roles: %v", err)
	}
	if actorRole != "admin" || targetRole != "owner" {
		t.Fatalf("roles = actor:%q target:%q, want admin/owner", actorRole, targetRole)
	}
}

func cleanupIAMSyncFixture(
	t *testing.T,
	database *postgres.DB,
	organizationID string,
	messageID string,
) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := database.Exec(
		ctx,
		"DELETE FROM public.platform_outbox_messages WHERE id = $1",
		messageID,
	); err != nil {
		t.Errorf("cleanup outbox: %v", err)
	}
	if _, err := database.Exec(
		ctx,
		"UPDATE iam.organizations SET status = 'disabled' WHERE id = $1::uuid",
		organizationID,
	); err != nil {
		t.Errorf("disable fixture organization: %v", err)
	}
	if _, err := database.Exec(
		ctx,
		"DELETE FROM iam.organizations WHERE id = $1::uuid",
		organizationID,
	); err != nil {
		t.Errorf("cleanup organization: %v", err)
	}
}
