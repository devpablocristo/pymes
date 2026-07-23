package iam

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	platformiam "github.com/devpablocristo/platform/iam/go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSecureMembershipResolverUsesOnlyVerifiedTuple(t *testing.T) {
	row := &resolverRow{values: []string{
		"00000000-0000-0000-0000-000000000201",
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000101",
		"owner",
	}}
	db := &resolverDB{row: row}
	session := platformiam.VerifiedSession{
		Provider:               "clerk",
		Subject:                "user_external",
		ExternalOrganizationID: "org_external",
	}

	active, err := (SecureMembershipResolver{}).ResolveActiveMembership(
		context.Background(),
		db,
		session,
	)
	if err != nil {
		t.Fatalf("ResolveActiveMembership() error = %v", err)
	}
	if active.Role != "owner" ||
		active.OrganizationID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("active membership = %+v", active)
	}
	if !strings.Contains(db.query, "iam.resolve_active_membership") {
		t.Fatalf("query = %q", db.query)
	}
	if want := []any{"clerk", "user_external", "org_external"}; !reflect.DeepEqual(
		db.args,
		want,
	) {
		t.Fatalf("query args = %#v, want %#v", db.args, want)
	}
}

func TestSecureMembershipResolverHidesMissingMembership(t *testing.T) {
	db := &resolverDB{row: &resolverRow{err: pgx.ErrNoRows}}
	_, err := (SecureMembershipResolver{}).ResolveActiveMembership(
		context.Background(),
		db,
		platformiam.VerifiedSession{
			Provider:               "clerk",
			Subject:                "unknown",
			ExternalOrganizationID: "unknown",
		},
	)
	if !errors.Is(err, platformiam.ErrActiveMembershipRequired) {
		t.Fatalf("error = %v, want ErrActiveMembershipRequired", err)
	}
}

type resolverDB struct {
	row   pgx.Row
	query string
	args  []any
}

func (*resolverDB) Exec(
	context.Context,
	string,
	...any,
) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected Exec")
}

func (*resolverDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query")
}

func (db *resolverDB) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	db.query = query
	db.args = append([]any(nil), args...)
	return db.row
}

type resolverRow struct {
	values []string
	err    error
}

func (row *resolverRow) Scan(dest ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(dest) != len(row.values) {
		return errors.New("unexpected destination count")
	}
	for index, value := range row.values {
		target, ok := dest[index].(*string)
		if !ok {
			return errors.New("unexpected destination type")
		}
		*target = value
	}
	return nil
}
