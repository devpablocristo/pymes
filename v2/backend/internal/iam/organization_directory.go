package iam

import (
	"context"
	"errors"
	"fmt"
	"strings"

	platformiam "github.com/devpablocristo/platform/iam/go"
	"github.com/jackc/pgx/v5"
)

// AccessibleOrganization is a local organization membership that an already
// verified external identity may activate. ExternalOrganizationID is opaque
// and is used only by the web client when calling Clerk.setActive().
type AccessibleOrganization struct {
	OrganizationID         string
	ExternalOrganizationID string
	Name                   string
	Slug                   string
	MembershipID           string
	Role                   Role
}

// OrganizationDirectory resolves the organizations admitted locally for a
// verified provider subject without trusting organization data from a request.
type OrganizationDirectory interface {
	ListActiveOrganizations(
		context.Context,
		string,
		string,
	) ([]AccessibleOrganization, error)
}

// SecureOrganizationDirectory crosses the pre-tenant RLS boundary only through
// the restricted SECURITY DEFINER function owned by Pymes.
type SecureOrganizationDirectory struct {
	db platformiam.DBTX
}

func NewSecureOrganizationDirectory(db platformiam.DBTX) (*SecureOrganizationDirectory, error) {
	if db == nil {
		return nil, fmt.Errorf("organization directory database is nil")
	}
	return &SecureOrganizationDirectory{db: db}, nil
}

func (directory *SecureOrganizationDirectory) ListActiveOrganizations(
	ctx context.Context,
	provider string,
	subject string,
) ([]AccessibleOrganization, error) {
	if directory == nil || directory.db == nil {
		return nil, fmt.Errorf("organization directory is not configured")
	}
	provider = strings.TrimSpace(provider)
	subject = strings.TrimSpace(subject)
	if provider == "" || subject == "" {
		return nil, fmt.Errorf("provider and subject are required")
	}

	rows, err := directory.db.Query(ctx, `
		SELECT
			organization_id::text,
			external_organization_id,
			organization_name,
			coalesce(organization_slug, ''),
			membership_id::text,
			role
		FROM iam.list_active_organizations($1, $2)
	`, provider, subject)
	if err != nil {
		return nil, fmt.Errorf("list active organizations: %w", err)
	}
	defer rows.Close()

	organizations := make([]AccessibleOrganization, 0)
	for rows.Next() {
		var (
			item    AccessibleOrganization
			rawRole string
		)
		if err := rows.Scan(
			&item.OrganizationID,
			&item.ExternalOrganizationID,
			&item.Name,
			&item.Slug,
			&item.MembershipID,
			&rawRole,
		); err != nil {
			return nil, fmt.Errorf("scan active organization: %w", err)
		}
		item.Role, err = ParseRole(rawRole)
		if err != nil {
			return nil, fmt.Errorf("scan active organization role: %w", err)
		}
		organizations = append(organizations, item)
	}
	if err := rows.Err(); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("iterate active organizations: %w", err)
	}
	return organizations, nil
}

var _ OrganizationDirectory = (*SecureOrganizationDirectory)(nil)
