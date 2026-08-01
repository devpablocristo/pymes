package organization

import (
	"context"
	"errors"
	"os"
	"testing"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFeatureFlagsAreTenantScopedVersionedAndAudited(t *testing.T) {
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
	runID := uuid.NewString()
	organizationAID := "org_feature_a_" + runID
	organizationBID := "org_feature_b_" + runID
	for _, organization := range []domain.Organization{
		{
			ID: organizationAID, Name: "Feature A",
			Slug: "feature-a-" + runID, Status: domain.Ready,
		},
		{
			ID: organizationBID, Name: "Feature B",
			Slug: "feature-b-" + runID, Status: domain.Ready,
		},
	} {
		if err := repository.SyncClerk(
			ctx,
			"clerk_"+organization.ID,
			organization,
		); err != nil {
			t.Fatal(err)
		}
	}
	initial, err := repository.GetFeatureFlags(ctx, organizationAID)
	if err != nil {
		t.Fatal(err)
	}
	if initial.SchedulingEnabled ||
		initial.WhatsAppEnabled ||
		initial.GoogleCalendarEnabled ||
		initial.FiscalRealEnabled ||
		initial.Version != 1 {
		t.Fatalf("unsafe defaults=%+v", initial)
	}
	updated, err := repository.UpdateFeatureFlags(
		ctx,
		domain.UpdateFeatureFlags{
			OrganizationID:        organizationAID,
			SchedulingEnabled:     true,
			WhatsAppEnabled:       true,
			GoogleCalendarEnabled: true,
			FiscalRealEnabled:     true,
			ExpectedVersion:       1,
			ActorID:               "owner-a",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 ||
		!updated.SchedulingEnabled ||
		!updated.WhatsAppEnabled ||
		!updated.GoogleCalendarEnabled ||
		!updated.FiscalRealEnabled {
		t.Fatalf("updated=%+v", updated)
	}
	if _, err = repository.UpdateFeatureFlags(
		ctx,
		domain.UpdateFeatureFlags{
			OrganizationID:  organizationAID,
			ExpectedVersion: 1,
			ActorID:         "stale-owner",
		},
	); !errors.Is(err, domain.ErrFeatureVersionConflict) {
		t.Fatalf("stale update err=%v", err)
	}
	other, err := repository.GetFeatureFlags(ctx, organizationBID)
	if err != nil {
		t.Fatal(err)
	}
	if other.SchedulingEnabled ||
		other.WhatsAppEnabled ||
		other.GoogleCalendarEnabled ||
		other.FiscalRealEnabled {
		t.Fatalf("cross-tenant state leaked=%+v", other)
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(
		ctx,
		"SELECT set_config('app.org_id',$1,true)",
		organizationAID,
	); err != nil {
		t.Fatal(err)
	}
	var auditCount int
	if err = tx.QueryRow(ctx, `
		SELECT count(*)
		FROM app.organization_feature_flag_audit
		WHERE org_id=$1`,
		organizationAID,
	).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 2 {
		t.Fatalf("audit count=%d", auditCount)
	}
	if _, err = tx.Exec(ctx, `
		UPDATE app.organization_feature_flag_audit
		SET changed_by='tampered'
		WHERE org_id=$1 AND version=2`,
		organizationAID,
	); err == nil {
		t.Fatal("feature audit accepted mutation")
	}
}
