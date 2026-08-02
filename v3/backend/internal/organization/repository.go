// Package repository contains PostgreSQL adapters for the organization directory.
// architecture:adapter repository
package organization

import (
	"context"
	"errors"
	"time"

	repositoryhelpers "github.com/devpablocristo/pymes/v3/backend/internal/organization/repository/helpers"
	repositorymodels "github.com/devpablocristo/pymes/v3/backend/internal/organization/repository/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func New(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool, now: time.Now} }

func (r *Postgres) ResolveBySlug(
	ctx context.Context,
	slug string,
) (domain.Organization, error) {
	var result domain.Organization
	err := r.pool.QueryRow(ctx, `
		SELECT id,name,slug,status,created_at,updated_at
		FROM app.organizations
		WHERE slug=$1`,
		slug,
	).Scan(
		&result.ID,
		&result.Name,
		&result.Slug,
		&result.Status,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Organization{}, domain.ErrUnknown
	}
	if err != nil {
		return domain.Organization{}, err
	}
	return result, nil
}

func (r *Postgres) Create(ctx context.Context, organization domain.Organization) (domain.Organization, error) {
	row := r.pool.QueryRow(ctx, `INSERT INTO app.organizations (id,name,slug,status,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$5) ON CONFLICT (id) DO UPDATE SET id=app.organizations.id RETURNING id,name,slug,status,created_at,updated_at`, organization.ID, organization.Name, organization.Slug, organization.Status, r.now().UTC())
	var result domain.Organization
	if err := row.Scan(&result.ID, &result.Name, &result.Slug, &result.Status, &result.CreatedAt, &result.UpdatedAt); err != nil {
		return domain.Organization{}, err
	}
	return result, nil
}
func (r *Postgres) SetStatus(ctx context.Context, organizationID string, status domain.Status) error {
	result, err := r.pool.Exec(ctx, `UPDATE app.organizations SET status=$1,updated_at=$2 WHERE id=$3`, status, r.now().UTC(), organizationID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return domain.ErrUnknown
	}
	return nil
}

func (r *Postgres) SetProvisioningStatus(ctx context.Context, organizationID, service, status, errorCode string) error {
	target := repositorymodels.ProvisioningTarget{Service: service, Status: status}
	column, err := repositoryhelpers.ProvisioningColumn(target)
	if err != nil {
		return err
	}
	query := `UPDATE app.organization_provisioning
		SET ` + column + `=$1,last_error=NULLIF($2,''),updated_at=$3
		WHERE organization_id=$4`
	result, err := r.pool.Exec(ctx, query, status, errorCode, r.now().UTC(), organizationID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return domain.ErrUnknown
	}
	return nil
}

func (r *Postgres) SyncClerk(ctx context.Context, clerkOrganizationID string, organization domain.Organization) error {
	if clerkOrganizationID == "" || organization.ID == "" {
		return errors.New("clerk organization and local organization are required")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := r.now().UTC()
	if _, err = tx.Exec(ctx, `INSERT INTO app.organizations (id,name,slug,status,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$5) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name,slug=EXCLUDED.slug,updated_at=EXCLUDED.updated_at`, organization.ID, organization.Name, organization.Slug, organization.Status, now); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO app.organization_identities(provider,provider_organization_id,org_id) VALUES ('clerk',$1,$2) ON CONFLICT (provider,provider_organization_id) DO UPDATE SET org_id=EXCLUDED.org_id`, clerkOrganizationID, organization.ID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO app.organization_provisioning
		  (organization_id,accounting_status,fiscal_status,updated_at)
		VALUES ($1,'pending','pending',$2)
		ON CONFLICT (organization_id) DO NOTHING`,
		organization.ID, now); err != nil {
		return err
	}
	if _, err = tx.Exec(
		ctx,
		"SELECT set_config('app.org_id',$1,true)",
		organization.ID,
	); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO app.organization_feature_flags (
		  org_id,scheduling_enabled,whatsapp_enabled,
		  google_calendar_enabled,fiscal_real_enabled,
		  version,updated_at,updated_by
		)
		VALUES ($1,false,false,false,false,1,$2,'system:provision')
		ON CONFLICT (org_id) DO NOTHING`,
		organization.ID, now); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO app.organization_feature_flag_audit (
		  org_id,version,scheduling_enabled,whatsapp_enabled,
		  google_calendar_enabled,fiscal_real_enabled,changed_by,changed_at
		)
		SELECT
		  org_id,version,scheduling_enabled,whatsapp_enabled,
		  google_calendar_enabled,fiscal_real_enabled,updated_by,updated_at
		FROM app.organization_feature_flags
		WHERE org_id=$1
		ON CONFLICT (org_id,version) DO NOTHING`,
		organization.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const featureFlagColumns = `
	org_id,scheduling_enabled,whatsapp_enabled,google_calendar_enabled,
	fiscal_real_enabled,version,updated_at,updated_by`

func (r *Postgres) GetFeatureFlags(
	ctx context.Context,
	organizationID string,
) (domain.FeatureFlags, error) {
	if r == nil || r.pool == nil {
		return domain.FeatureFlags{}, errors.New("organization database is required")
	}
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return domain.FeatureFlags{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	flags, err := repositoryhelpers.ScanFeatureFlags(tx.QueryRow(ctx, `
		SELECT `+featureFlagColumns+`
		FROM app.organization_feature_flags
		WHERE org_id=$1`,
		organizationID,
	))
	if err != nil {
		return domain.FeatureFlags{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.FeatureFlags{}, err
	}
	return flags, nil
}

func (r *Postgres) UpdateFeatureFlags(
	ctx context.Context,
	command domain.UpdateFeatureFlags,
) (domain.FeatureFlags, error) {
	if r == nil || r.pool == nil || !command.Valid() {
		return domain.FeatureFlags{}, errors.New("valid organization feature update is required")
	}
	tx, err := repositoryhelpers.BeginTenant(
		ctx,
		r.pool,
		command.OrganizationID,
	)
	if err != nil {
		return domain.FeatureFlags{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := r.now().UTC()
	flags, err := repositoryhelpers.ScanFeatureFlags(tx.QueryRow(ctx, `
		UPDATE app.organization_feature_flags
		SET scheduling_enabled=$2,
		    whatsapp_enabled=$3,
		    google_calendar_enabled=$4,
		    fiscal_real_enabled=$5,
		    version=version+1,
		    updated_at=$6,
		    updated_by=$7
		WHERE org_id=$1 AND version=$8
		RETURNING `+featureFlagColumns,
		command.OrganizationID,
		command.SchedulingEnabled,
		command.WhatsAppEnabled,
		command.GoogleCalendarEnabled,
		command.FiscalRealEnabled,
		now,
		command.ActorID,
		command.ExpectedVersion,
	))
	if errors.Is(err, domain.ErrUnknown) {
		return domain.FeatureFlags{}, domain.ErrFeatureVersionConflict
	}
	if err != nil {
		return domain.FeatureFlags{}, err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO app.organization_feature_flag_audit (
		  org_id,version,scheduling_enabled,whatsapp_enabled,
		  google_calendar_enabled,fiscal_real_enabled,changed_by,changed_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		flags.OrganizationID,
		flags.Version,
		flags.SchedulingEnabled,
		flags.WhatsAppEnabled,
		flags.GoogleCalendarEnabled,
		flags.FiscalRealEnabled,
		flags.UpdatedBy,
		flags.UpdatedAt,
	); err != nil {
		return domain.FeatureFlags{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.FeatureFlags{}, err
	}
	return flags, nil
}
