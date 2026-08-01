package organization

import (
	"context"
	"os"
	"testing"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProvisioningStateIsExplicitAndIdempotent(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repository := New(pool)
	organization := domain.Organization{ID: "org_provisioning_test", Name: "Provisioning", Slug: "provisioning-test", Status: domain.Pending}
	if err := repository.SyncClerk(ctx, "org_clerk_provisioning", organization); err != nil {
		t.Fatal(err)
	}
	if err := repository.SetProvisioningStatus(ctx, organization.ID, "accounting", "ready", ""); err != nil {
		t.Fatal(err)
	}
	if err := repository.SetProvisioningStatus(ctx, organization.ID, "fiscal", "ready", ""); err != nil {
		t.Fatal(err)
	}
	organization.Name = "Provisioning renamed"
	if err := repository.SyncClerk(ctx, "org_clerk_provisioning", organization); err != nil {
		t.Fatal(err)
	}
	var accountingStatus, fiscalStatus string
	if err := pool.QueryRow(ctx, `
		SELECT accounting_status,fiscal_status
		FROM app.organization_provisioning
		WHERE organization_id=$1`, organization.ID).Scan(&accountingStatus, &fiscalStatus); err != nil {
		t.Fatal(err)
	}
	if accountingStatus != "ready" || fiscalStatus != "ready" {
		t.Fatalf("unexpected provisioning state accounting=%s fiscal=%s", accountingStatus, fiscalStatus)
	}
	if err := repository.SetProvisioningStatus(ctx, organization.ID, "unknown", "ready", ""); err == nil {
		t.Fatal("unknown provisioning service must fail")
	}
}
