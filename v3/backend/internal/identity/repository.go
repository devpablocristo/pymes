// architecture:adapter repository
package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
	repositoryhelpers "github.com/devpablocristo/pymes/v3/backend/internal/identity/repository/helpers"
	repositorymodels "github.com/devpablocristo/pymes/v3/backend/internal/identity/repository/models"
	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres is the identity adapter. It is deliberately independent from
// commerce persistence: the use-case only knows its inbox port.
type Postgres struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

func (r *Postgres) ResolveClerkMembership(ctx context.Context, clerkOrganizationID, clerkUserID string) (identitydomain.Principal, error) {
	if clerkOrganizationID == "" || clerkUserID == "" {
		return identitydomain.Principal{}, fmt.Errorf("organization membership is required")
	}
	var organizationID string
	if err := r.pool.QueryRow(ctx, `SELECT org_id FROM app.organization_identities WHERE provider='clerk' AND provider_organization_id=$1`, clerkOrganizationID).Scan(&organizationID); err != nil {
		return identitydomain.Principal{}, fmt.Errorf("resolve clerk organization: %w", err)
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return identitydomain.Principal{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		return identitydomain.Principal{}, err
	}
	principal := identitydomain.Principal{OrganizationID: organizationID, ActorID: clerkUserID}
	var membership repositorymodels.Membership
	if err = tx.QueryRow(ctx, `
		SELECT m.role,m.permissions::text,m.status,o.status
		FROM app.memberships m
		JOIN app.organizations o ON o.id=m.org_id
		WHERE m.org_id=$1 AND m.provider='clerk' AND m.provider_user_id=$2`,
		organizationID, clerkUserID).Scan(
		&membership.Role, &membership.PermissionsJSON, &membership.Status, &membership.OrganizationStatus,
	); err != nil {
		return identitydomain.Principal{}, fmt.Errorf("resolve clerk membership: %w", err)
	}
	principal.MembershipStatus = membership.Status
	principal.OrganizationStatus = membership.OrganizationStatus
	if principal.MembershipStatus != "active" {
		return identitydomain.Principal{}, fmt.Errorf("resolve clerk membership: inactive")
	}
	principal.Role = identitydomain.Role(membership.Role)
	if err = json.Unmarshal([]byte(membership.PermissionsJSON), &principal.Permissions); err != nil {
		return identitydomain.Principal{}, fmt.Errorf("resolve clerk permissions: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return identitydomain.Principal{}, err
	}
	return principal, nil
}

func (r *Postgres) Receive(ctx context.Context, event Event) (bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `INSERT INTO app.clerk_webhook_inbox(event_id,event_type,occurred_at,payload,processed_at) VALUES($1,$2,$3,$4,now()) ON CONFLICT(event_id) DO NOTHING`, event.ID, event.Type, event.OccurredAt, event.Payload)
	if err != nil {
		return false, err
	}
	if result.RowsAffected() == 0 {
		return true, tx.Commit(ctx)
	}
	if err := r.project(ctx, tx, event); err != nil {
		return false, err
	}
	return false, tx.Commit(ctx)
}

func (r *Postgres) project(ctx context.Context, tx pgx.Tx, event Event) error {
	decoded, err := clerk.DecodeWebhookEvent(event.Payload)
	if err != nil {
		return err
	}
	switch data := decoded.Data.(type) {
	case *clerk.Organization:
		return r.organization(
			ctx,
			tx,
			*data,
			decoded.Type == clerk.WebhookOrganizationDeleted,
			decoded.Type == clerk.WebhookOrganizationCreated,
		)
	case *clerk.OrganizationMembership:
		return r.membership(ctx, tx, *data, decoded.Type == clerk.WebhookOrganizationMembershipDeleted)
	case *clerk.DeletedResource:
		if decoded.Type == clerk.WebhookOrganizationDeleted {
			_, err := tx.Exec(ctx, `UPDATE app.organizations SET status='suspended',updated_at=now() WHERE id=(SELECT org_id FROM app.organization_identities WHERE provider='clerk' AND provider_organization_id=$1)`, data.ID)
			return err
		}
	}
	return nil
}
func (r *Postgres) organization(ctx context.Context, tx pgx.Tx, value clerk.Organization, deleted, created bool) error {
	status := "pending"
	if deleted {
		status = "suspended"
	}
	localID := "org_" + strings.ReplaceAll(value.ID, "-", "_")
	_, err := tx.Exec(ctx, `
		INSERT INTO app.organizations(id,name,slug,status,created_at,updated_at)
		VALUES($1,$2,$3,$4,now(),now())
		ON CONFLICT(id) DO UPDATE SET
		  name=EXCLUDED.name,
		  slug=EXCLUDED.slug,
		  status=CASE WHEN $5 THEN EXCLUDED.status ELSE app.organizations.status END,
		  updated_at=now()`,
		localID, value.Name, value.Slug, status, deleted || created)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO app.organization_identities(provider,provider_organization_id,org_id) VALUES('clerk',$1,$2) ON CONFLICT(provider,provider_organization_id) DO UPDATE SET org_id=EXCLUDED.org_id`, value.ID, localID)
	return err
}
func (r *Postgres) membership(ctx context.Context, tx pgx.Tx, value clerk.OrganizationMembership, deleted bool) error {
	var orgID string
	if err := tx.QueryRow(ctx, `SELECT org_id FROM app.organization_identities WHERE provider='clerk' AND provider_organization_id=$1`, value.OrganizationID).Scan(&orgID); err != nil {
		return fmt.Errorf("membership organization not projected: %w", err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", orgID); err != nil {
		return err
	}
	status := "active"
	if deleted {
		status = "inactive"
	}
	role := repositoryhelpers.LocalRole(value.Role)
	permissions, _ := json.Marshal(value.Permissions)
	_, err := tx.Exec(ctx, `INSERT INTO app.memberships(org_id,provider,provider_user_id,role,permissions,status,updated_at) VALUES($1,'clerk',$2,$3,$4,$5,now()) ON CONFLICT(org_id,provider,provider_user_id) DO UPDATE SET role=EXCLUDED.role,permissions=EXCLUDED.permissions,status=EXCLUDED.status,updated_at=now()`, orgID, value.User.ID, role, permissions, status)
	return err
}

func localRole(value string) string {
	return repositoryhelpers.LocalRole(value)
}
