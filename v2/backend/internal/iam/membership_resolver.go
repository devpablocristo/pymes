package iam

import (
	"context"
	"errors"
	"fmt"

	platformiam "github.com/devpablocristo/platform/iam/go"
	"github.com/jackc/pgx/v5"
)

// SecureMembershipResolver crosses the RLS bootstrap boundary exclusively
// through Pymes' restricted SECURITY DEFINER function. It never resolves
// tenancy from request metadata.
type SecureMembershipResolver struct{}

func (SecureMembershipResolver) ResolveActiveMembership(
	ctx context.Context,
	db platformiam.DBTX,
	session platformiam.VerifiedSession,
) (platformiam.ActiveMembership, error) {
	if db == nil {
		return platformiam.ActiveMembership{}, platformiam.ErrActiveMembershipRequired
	}

	var active platformiam.ActiveMembership
	err := db.QueryRow(ctx, `
		SELECT
			membership_id::text,
			organization_id::text,
			user_id::text,
			role
		FROM iam.resolve_active_membership($1, $2, $3)
	`,
		session.Provider,
		session.Subject,
		session.ExternalOrganizationID,
	).Scan(
		&active.MembershipID,
		&active.OrganizationID,
		&active.UserID,
		&active.Role,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return platformiam.ActiveMembership{}, platformiam.ErrActiveMembershipRequired
	}
	if err != nil {
		return platformiam.ActiveMembership{}, fmt.Errorf("resolve Pymes membership: %w", err)
	}
	return active, nil
}

var _ platformiam.MembershipResolver = SecureMembershipResolver{}
