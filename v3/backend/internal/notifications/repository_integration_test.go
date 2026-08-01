package notifications

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func notificationTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is not configured")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func enableNotifications(
	t *testing.T,
	pool *pgxpool.Pool,
	organizationID string,
) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		INSERT INTO app.organizations(id,name,slug,status)
		VALUES($1,$2,$3,'ready')
		ON CONFLICT (id) DO UPDATE SET status='ready'`,
		organizationID, organizationID, organizationID,
	)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(
		ctx,
		"SELECT set_config('app.org_id',$1,true)",
		organizationID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO app.notification_settings(org_id,whatsapp_enabled)
		VALUES($1,true)
		ON CONFLICT (org_id) DO UPDATE SET whatsapp_enabled=true`,
		organizationID,
	); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func integrationIntent(
	organizationID string,
	notificationID string,
	idempotencyKey string,
) domain.Intent {
	return domain.Intent{
		ID: notificationID, OrganizationID: organizationID,
		Kind: domain.KindConfirmation, AggregateType: "booking",
		AggregateID: "booking-1", RecipientE164: "+5491112345678",
		TemplateName: "booking.confirmation", TemplateVersion: 1,
		Locale: "es_AR", Variables: map[string]string{"customer": "Pablo"},
		Body:           "Tu turno está confirmado.",
		SendAt:         time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
		Status:         domain.StatusPending,
		IdempotencyKey: idempotencyKey, CorrelationID: "correlation-1",
		RequestID: "request-1", ActorRef: "system:scheduling",
		SourceVersion: 1,
	}
}

func TestPostgresNotificationIdempotencyRLSAndWebhookInbox(
	t *testing.T,
) {
	pool := notificationTestPool(t)
	organizationA := "notification-org-a-" + uuid.NewString()
	organizationB := "notification-org-b-" + uuid.NewString()
	enableNotifications(t, pool, organizationA)
	enableNotifications(t, pool, organizationB)
	repository := NewPostgres(pool)
	request := RequestNotification{Repository: repository}
	ctx := context.Background()
	intentA := integrationIntent(
		organizationA, "shared-notification", "confirmation-1",
	)
	first, err := request.Execute(ctx, intentA)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := request.Execute(ctx, intentA)
	if err != nil || replay.ID != first.ID {
		t.Fatalf("idempotent replay = %+v, %v", replay, err)
	}
	changed := intentA
	changed.Body = "different payload"
	if _, err = request.Execute(ctx, changed); !errors.Is(
		err,
		domain.ErrIdempotencyKeyReused,
	) {
		t.Fatalf("changed replay error = %v", err)
	}
	intentB := integrationIntent(
		organizationB, "shared-notification", "confirmation-1",
	)
	if _, err = request.Execute(ctx, intentB); err != nil {
		t.Fatalf("same IDs in another tenant: %v", err)
	}

	event := domain.DeliveryEvent{
		Event: "message.sent", TraceID: "pymes.v1.trace",
		NotificationID: first.ID,
		MessageID:      "pergo-message-1", Channel: "whatsapp",
		Timestamp: time.Now().UTC(), WorkspaceID: "workspace-1",
	}
	duplicate, err := repository.ApplyDeliveryEvent(
		ctx, intentA.OrganizationID, event,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	)
	if err != nil || duplicate {
		t.Fatalf("first webhook duplicate=%v err=%v", duplicate, err)
	}
	duplicate, err = repository.ApplyDeliveryEvent(
		ctx, intentA.OrganizationID, event,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	)
	if err != nil || !duplicate {
		t.Fatalf("replayed webhook duplicate=%v err=%v", duplicate, err)
	}
	stored, err := repository.Get(ctx, intentA.OrganizationID, first.ID)
	if err != nil || stored.Status != domain.StatusSent ||
		stored.ExternalMessageID != event.MessageID {
		t.Fatalf("stored notification=%+v err=%v", stored, err)
	}
	if _, err = repository.Get(
		ctx, organizationB, "only-org-a",
	); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant lookup error = %v", err)
	}
}

func TestPostgresNotificationConcurrentReplayCreatesOneIntentAndOutbox(
	t *testing.T,
) {
	pool := notificationTestPool(t)
	organizationID := "notification-org-concurrent-" + uuid.NewString()
	enableNotifications(t, pool, organizationID)
	repository := NewPostgres(pool)
	request := RequestNotification{Repository: repository}
	intent := integrationIntent(
		organizationID, "notification-concurrent", "confirmation-concurrent",
	)
	const workers = 8
	var wait sync.WaitGroup
	errorsChannel := make(chan error, workers)
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := request.Execute(context.Background(), intent)
			errorsChannel <- err
		}()
	}
	wait.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Fatal(err)
		}
	}
	tx, err := pool.BeginTx(context.Background(), pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(
		context.Background(),
		"SELECT set_config('app.org_id',$1,true)",
		organizationID,
	); err != nil {
		t.Fatal(err)
	}
	var intents, events int
	if err = tx.QueryRow(
		context.Background(),
		`SELECT count(*) FROM app.notifications
		 WHERE org_id=$1 AND idempotency_key=$2`,
		organizationID, intent.IdempotencyKey,
	).Scan(&intents); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(
		context.Background(),
		`SELECT count(*) FROM app.outbox
		 WHERE org_id=$1 AND topic='NotificationRequested'
		   AND idempotency_key=$2`,
		organizationID, intent.IdempotencyKey,
	).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if intents != 1 || events != 1 {
		t.Fatalf("intents=%d events=%d", intents, events)
	}
}
