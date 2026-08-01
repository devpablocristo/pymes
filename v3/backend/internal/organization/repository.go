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
	return tx.Commit(ctx)
}
