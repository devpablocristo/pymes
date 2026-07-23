package provisioning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestProvisionIsTransactionalIdempotentAndConcurrent(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	database, err := postgres.OpenWithConfig(
		ctx,
		databaseURL,
		postgres.DefaultConfig("pymes-v2-provisioning-test"),
	)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer database.Close()

	service, err := NewService(database.Pool())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	t.Run("same payload replays one durable result", func(t *testing.T) {
		slug := uniqueSlug("replay")
		input := Input{
			Name:       "Replay Organization",
			Slug:       slug,
			OwnerEmail: "owner+" + slug + "@example.test",
		}
		t.Cleanup(func() { cleanupProvisioning(t, database, slug) })

		first, err := service.Provision(ctx, input)
		if err != nil {
			t.Fatalf("first Provision() error = %v", err)
		}
		repeated, err := service.Provision(ctx, input)
		if err != nil {
			t.Fatalf("repeated Provision() error = %v", err)
		}
		if !reflect.DeepEqual(repeated, first) {
			t.Fatalf("replayed result = %#v, want %#v", repeated, first)
		}

		assertProvisioningState(t, ctx, database, first)

		_, err = service.Provision(ctx, Input{
			Name:       "Different Organization",
			Slug:       slug,
			OwnerEmail: input.OwnerEmail,
		})
		if !errors.Is(err, ErrPayloadConflict) {
			t.Fatalf("changed payload error = %v, want ErrPayloadConflict", err)
		}
		assertProvisioningState(t, ctx, database, first)
	})

	t.Run("concurrent requests converge on one result", func(t *testing.T) {
		slug := uniqueSlug("concurrent")
		input := Input{
			Name:       "Concurrent Organization",
			Slug:       slug,
			OwnerEmail: "owner+" + slug + "@example.test",
		}
		t.Cleanup(func() { cleanupProvisioning(t, database, slug) })

		const workers = 8
		results := make([]Result, workers)
		errs := make([]error, workers)
		var waitGroup sync.WaitGroup
		for index := range workers {
			waitGroup.Add(1)
			go func() {
				defer waitGroup.Done()
				results[index], errs[index] = service.Provision(ctx, input)
			}()
		}
		waitGroup.Wait()

		for index := range workers {
			if errs[index] != nil {
				t.Fatalf("Provision() worker %d error = %v", index, errs[index])
			}
			if !reflect.DeepEqual(results[index], results[0]) {
				t.Fatalf("worker %d result = %#v, want %#v", index, results[index], results[0])
			}
		}
		assertProvisioningState(t, ctx, database, results[0])
	})

	t.Run("outbox failure rolls back request and organization", func(t *testing.T) {
		slug := uniqueSlug("rollback")
		t.Cleanup(func() { cleanupProvisioning(t, database, slug) })

		uow, err := postgres.NewPgxUnitOfWork(database.Pool())
		if err != nil {
			t.Fatalf("NewPgxUnitOfWork() error = %v", err)
		}
		failing, err := newService(uow, failingAppender{}, uuidGenerator{})
		if err != nil {
			t.Fatalf("newService() error = %v", err)
		}

		_, err = failing.Provision(ctx, Input{
			Name:       "Rollback Organization",
			Slug:       slug,
			OwnerEmail: "owner+" + slug + "@example.test",
		})
		if err == nil {
			t.Fatal("Provision() error = nil, want outbox failure")
		}

		assertCountForSlug(t, ctx, database, "app.organization_provisioning_requests", "slug", slug, 0)
		assertCountForSlug(t, ctx, database, "iam.organizations", "slug", slug, 0)
		assertCountForSlug(
			t,
			ctx,
			database,
			"public.platform_outbox_messages",
			"idempotency_key",
			"iam.provision-org:"+slug,
			0,
		)
	})
}

type failingAppender struct{}

func (failingAppender) Append(
	context.Context,
	pgx.Tx,
	platformoutbox.MessageInput,
) (platformoutbox.Message, error) {
	return platformoutbox.Message{}, errors.New("synthetic outbox failure")
}

func uniqueSlug(prefix string) string {
	return prefix + "-" + uuid.NewString()[:8]
}

func cleanupProvisioning(t *testing.T, database *postgres.DB, slug string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := database.Exec(
		ctx,
		"DELETE FROM public.platform_outbox_messages WHERE idempotency_key = $1",
		"iam.provision-org:"+slug,
	); err != nil {
		t.Errorf("cleanup outbox: %v", err)
	}
	if _, err := database.Exec(
		ctx,
		"DELETE FROM app.organization_provisioning_requests WHERE slug = $1",
		slug,
	); err != nil {
		t.Errorf("cleanup request: %v", err)
	}
	if _, err := database.Exec(ctx, "DELETE FROM iam.organizations WHERE slug = $1", slug); err != nil {
		t.Errorf("cleanup organization: %v", err)
	}
}

func assertProvisioningState(
	t *testing.T,
	ctx context.Context,
	database *postgres.DB,
	result Result,
) {
	t.Helper()

	assertCountForSlug(t, ctx, database, "app.organization_provisioning_requests", "slug", result.Slug, 1)
	assertCountForSlug(t, ctx, database, "iam.organizations", "slug", result.Slug, 1)
	assertCountForSlug(
		t,
		ctx,
		database,
		"public.platform_outbox_messages",
		"idempotency_key",
		"iam.provision-org:"+result.Slug,
		1,
	)

	var organizationStatus string
	if err := database.QueryRow(
		ctx,
		"SELECT status FROM iam.organizations WHERE id = $1::uuid",
		result.OrganizationID,
	).Scan(&organizationStatus); err != nil {
		t.Fatalf("query organization status: %v", err)
	}
	if organizationStatus != "provisioning" {
		t.Fatalf("organization status = %q, want provisioning", organizationStatus)
	}

	var topic string
	var payload []byte
	if err := database.QueryRow(
		ctx,
		`SELECT topic, payload
		   FROM public.platform_outbox_messages
		  WHERE id = $1`,
		result.OutboxMessageID,
	).Scan(&topic, &payload); err != nil {
		t.Fatalf("query outbox message: %v", err)
	}
	if topic != ProvisionOrganizationTopic {
		t.Fatalf("outbox topic = %q, want %q", topic, ProvisionOrganizationTopic)
	}
	var event struct {
		RequestID      string `json:"request_id"`
		OrganizationID string `json:"organization_id"`
		OwnerEmail     string `json:"owner_email"`
		OwnerRole      string `json:"owner_role"`
		ProviderRole   string `json:"provider_role"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatalf("decode outbox payload: %v", err)
	}
	if event.RequestID != result.RequestID ||
		event.OrganizationID != result.OrganizationID ||
		event.OwnerEmail != result.OwnerEmail ||
		event.OwnerRole != "owner" ||
		event.ProviderRole != "org:admin" {
		t.Fatalf("outbox event = %#v, result = %#v", event, result)
	}
}

func assertCountForSlug(
	t *testing.T,
	ctx context.Context,
	database *postgres.DB,
	table string,
	column string,
	value string,
	want int,
) {
	t.Helper()

	var count int
	query := fmt.Sprintf("SELECT count(*) FROM %s WHERE %s = $1", table, column)
	if err := database.QueryRow(ctx, query, value).Scan(&count); err != nil {
		t.Fatalf("query %s count: %v", table, err)
	}
	if count != want {
		t.Fatalf("%s count = %d, want %d", table, count, want)
	}
}
