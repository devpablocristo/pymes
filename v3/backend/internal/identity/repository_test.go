package identity

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

func TestPostgresProjectsClerkOrganizationAndMembershipIdempotently(t *testing.T) {
	url := os.Getenv("PYMES_DATABASE_TEST_URL")
	if url == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(context.Background(), `TRUNCATE app.clerk_webhook_inbox,app.memberships,app.organization_identities,app.organizations CASCADE`); err != nil {
		t.Fatal(err)
	}
	repo := New(pool)
	now := time.Now().UTC()
	organization := []byte(`{"data":{"id":"org_clerk","name":"Clerk Org","slug":"clerk-org","created_at":1784818800000,"updated_at":1784818800000},"object":"event","type":"organization.created","timestamp":1784818800000,"instance_id":"ins"}`)
	if duplicate, err := repo.Receive(context.Background(), Event{ID: "evt-org", Type: "organization.created", OccurredAt: now, Payload: organization}); err != nil || duplicate {
		t.Fatalf("org duplicate=%v err=%v", duplicate, err)
	}
	membership := []byte(`{"data":{"id":"orgmem","role":"org:admin","permissions":["org:members:read"],"organization":{"id":"org_clerk","name":"Clerk Org","slug":"clerk-org"},"public_user_data":{"user_id":"user_clerk","identifier":"u@example.com"},"created_at":1784818800000,"updated_at":1784818800000},"object":"event","type":"organizationMembership.created","timestamp":1784818800000,"instance_id":"ins"}`)
	if duplicate, err := repo.Receive(context.Background(), Event{ID: "evt-member", Type: "organizationMembership.created", OccurredAt: now, Payload: membership}); err != nil || duplicate {
		t.Fatalf("member duplicate=%v err=%v", duplicate, err)
	}
	if duplicate, err := repo.Receive(context.Background(), Event{ID: "evt-member", Type: "organizationMembership.created", OccurredAt: now, Payload: membership}); err != nil || !duplicate {
		t.Fatalf("retry duplicate=%v err=%v", duplicate, err)
	}
	var status, role string
	if err := pool.QueryRow(context.Background(), `SELECT m.status,m.role FROM app.memberships m JOIN app.organization_identities i ON i.org_id=m.org_id WHERE i.provider_organization_id='org_clerk' AND m.provider_user_id='user_clerk'`).Scan(&status, &role); err != nil {
		t.Fatal(err)
	}
	if status != "active" || role != "admin" {
		t.Fatalf("status=%s role=%s", status, role)
	}
	if principal, err := repo.ResolveClerkMembership(context.Background(), "org_clerk", "user_clerk"); err != nil ||
		principal.OrganizationID != "org_org_clerk" || principal.ActorID != "user_clerk" ||
		principal.Role != "admin" || len(principal.Permissions) != 1 || principal.MembershipStatus != "active" ||
		principal.OrganizationStatus != "pending" {
		t.Fatalf("resolved=%+v err=%v", principal, err)
	}
	if _, err := repo.ResolveClerkMembership(context.Background(), "org_clerk", "other_user"); err == nil {
		t.Fatal("unknown membership must not authorize")
	}
	if _, err := pool.Exec(context.Background(), `UPDATE app.organizations SET status='ready' WHERE id='org_org_clerk'`); err != nil {
		t.Fatal(err)
	}
	organizationUpdated := []byte(`{"data":{"id":"org_clerk","name":"Clerk Org Renamed","slug":"clerk-org","created_at":1784818800000,"updated_at":1784818801000},"object":"event","type":"organization.updated","timestamp":1784818801000,"instance_id":"ins"}`)
	if duplicate, err := repo.Receive(context.Background(), Event{ID: "evt-org-updated", Type: "organization.updated", OccurredAt: now, Payload: organizationUpdated}); err != nil || duplicate {
		t.Fatalf("org update duplicate=%v err=%v", duplicate, err)
	}
	var organizationStatus, organizationName string
	if err := pool.QueryRow(context.Background(), `SELECT status,name FROM app.organizations WHERE id='org_org_clerk'`).Scan(&organizationStatus, &organizationName); err != nil {
		t.Fatal(err)
	}
	if organizationStatus != "ready" || organizationName != "Clerk Org Renamed" {
		t.Fatalf("organization status=%s name=%s", organizationStatus, organizationName)
	}
	membershipDeleted := []byte(`{"data":{"id":"orgmem","role":"org:admin","permissions":["org:members:read"],"organization":{"id":"org_clerk","name":"Clerk Org","slug":"clerk-org"},"public_user_data":{"user_id":"user_clerk","identifier":"u@example.com"},"created_at":1784818800000,"updated_at":1784818802000},"object":"event","type":"organizationMembership.deleted","timestamp":1784818802000,"instance_id":"ins"}`)
	if duplicate, err := repo.Receive(context.Background(), Event{ID: "evt-member-deleted", Type: "organizationMembership.deleted", OccurredAt: now, Payload: membershipDeleted}); err != nil || duplicate {
		t.Fatalf("membership delete duplicate=%v err=%v", duplicate, err)
	}
	if _, err := repo.ResolveClerkMembership(context.Background(), "org_clerk", "user_clerk"); err == nil {
		t.Fatal("deleted membership must not authorize")
	}
}
