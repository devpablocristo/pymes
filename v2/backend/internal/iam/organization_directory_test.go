package iam

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSecureOrganizationDirectoryUsesVerifiedIdentityTuple(t *testing.T) {
	db := &organizationDirectoryDB{
		rows: &organizationRows{
			values: [][]string{{
				"00000000-0000-0000-0000-000000000001",
				"org_external",
				"Acme",
				"acme",
				"00000000-0000-0000-0000-000000000201",
				"owner",
			}},
		},
	}
	directory, err := NewSecureOrganizationDirectory(db)
	if err != nil {
		t.Fatalf("NewSecureOrganizationDirectory() error = %v", err)
	}

	organizations, err := directory.ListActiveOrganizations(
		context.Background(),
		" clerk ",
		" user_external ",
	)
	if err != nil {
		t.Fatalf("ListActiveOrganizations() error = %v", err)
	}
	if len(organizations) != 1 || organizations[0].Role != RoleOwner ||
		organizations[0].ExternalOrganizationID != "org_external" {
		t.Fatalf("organizations = %+v", organizations)
	}
	if !strings.Contains(db.query, "iam.list_active_organizations") {
		t.Fatalf("query = %q", db.query)
	}
	if want := []any{"clerk", "user_external"}; !reflect.DeepEqual(db.args, want) {
		t.Fatalf("query args = %#v, want %#v", db.args, want)
	}
}

func TestSecureOrganizationDirectoryFailsClosed(t *testing.T) {
	if _, err := NewSecureOrganizationDirectory(nil); err == nil {
		t.Fatal("expected nil database error")
	}
	db := &organizationDirectoryDB{rows: &organizationRows{}}
	directory, err := NewSecureOrganizationDirectory(db)
	if err != nil {
		t.Fatalf("NewSecureOrganizationDirectory() error = %v", err)
	}
	if _, err := directory.ListActiveOrganizations(context.Background(), "", "subject"); err == nil {
		t.Fatal("expected empty provider error")
	}
}

type organizationDirectoryDB struct {
	rows  pgx.Rows
	query string
	args  []any
}

func (*organizationDirectoryDB) Exec(
	context.Context,
	string,
	...any,
) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected Exec")
}

func (db *organizationDirectoryDB) Query(
	_ context.Context,
	query string,
	args ...any,
) (pgx.Rows, error) {
	db.query = query
	db.args = append([]any(nil), args...)
	return db.rows, nil
}

func (*organizationDirectoryDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return &resolverRow{err: errors.New("unexpected QueryRow")}
}

type organizationRows struct {
	values [][]string
	index  int
	err    error
}

func (rows *organizationRows) Close()                                       {}
func (rows *organizationRows) Err() error                                   { return rows.err }
func (rows *organizationRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (rows *organizationRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (rows *organizationRows) Values() ([]any, error)                       { return nil, nil }
func (rows *organizationRows) RawValues() [][]byte                          { return nil }
func (rows *organizationRows) Conn() *pgx.Conn                              { return nil }

func (rows *organizationRows) Next() bool {
	return rows.index < len(rows.values)
}

func (rows *organizationRows) Scan(dest ...any) error {
	if rows.index >= len(rows.values) {
		return errors.New("scan called without a row")
	}
	values := rows.values[rows.index]
	rows.index++
	if len(values) != len(dest) {
		return errors.New("unexpected destination count")
	}
	for index, value := range values {
		target, ok := dest[index].(*string)
		if !ok {
			return errors.New("unexpected destination type")
		}
		*target = value
	}
	return nil
}
