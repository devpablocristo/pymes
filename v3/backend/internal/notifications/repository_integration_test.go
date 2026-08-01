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
		INSERT INTO app.organization_feature_flags(
		  org_id,whatsapp_enabled,updated_by
		)
		VALUES($1,true,'test')
		ON CONFLICT (org_id) DO UPDATE
		  SET whatsapp_enabled=true,updated_by='test'`,
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

func TestPostgresSchedulingProjectionIsIdempotentWithoutASecondOutbox(
	t *testing.T,
) {
	pool := notificationTestPool(t)
	organizationID := "notification-org-projection-" + uuid.NewString()
	enableNotifications(t, pool, organizationID)
	repository := NewPostgres(pool)
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	project := ProjectSchedulingNotification{
		Repository: repository,
		Clock:      func() time.Time { return now },
	}
	metadata := ProjectionMetadata{
		EventID:        uuid.NewString(),
		OrganizationID: organizationID,
		IdempotencyKey: "scheduling:NotificationRequested:booking-1:source-1:1",
		CorrelationID:  "correlation-projection",
		RequestID:      "request-projection",
		ActorRef:       "system:scheduling",
		SourceVersion:  1,
	}
	input := SchedulingNotification{
		Trigger: "BookingConfirmed", AggregateType: "booking",
		AggregateID: "booking-1", RecipientE164: "+5491112345678",
		CustomerName: "Ada", ServiceName: "Consulta",
		StartAt: now.Add(24 * time.Hour), EndAt: now.Add(25 * time.Hour),
		Timezone: "America/Argentina/Buenos_Aires",
	}
	first, deliver, err := project.Execute(
		context.Background(), metadata, input,
	)
	if err != nil || !deliver {
		t.Fatalf("first projection deliver=%v err=%v", deliver, err)
	}
	replayed, deliver, err := project.Execute(
		context.Background(), metadata, input,
	)
	if err != nil || !deliver || replayed.ID != first.ID {
		t.Fatalf(
			"replay projection=%+v first=%+v deliver=%v err=%v",
			replayed,
			first,
			deliver,
			err,
		)
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
	var intents, selfQueued int
	if err = tx.QueryRow(context.Background(), `
		SELECT count(*) FROM app.notifications
		WHERE org_id=$1 AND idempotency_key=$2`,
		organizationID,
		metadata.IdempotencyKey,
	).Scan(&intents); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(context.Background(), `
		SELECT count(*) FROM app.outbox
		WHERE org_id=$1 AND topic='NotificationRequested'
		  AND idempotency_key=$2`,
		organizationID,
		metadata.IdempotencyKey,
	).Scan(&selfQueued); err != nil {
		t.Fatal(err)
	}
	if intents != 1 || selfQueued != 0 {
		t.Fatalf("intents=%d self_queued=%d", intents, selfQueued)
	}

	input.CustomerName = "Grace"
	_, _, err = project.Execute(context.Background(), metadata, input)
	if !errors.Is(err, domain.ErrIdempotencyKeyReused) {
		t.Fatalf("changed replay error=%v", err)
	}
}

func TestPostgresNotificationLeaseCannotStealAnotherContextEvent(
	t *testing.T,
) {
	pool := notificationTestPool(t)
	organizationID := "notification-org-lease-" + uuid.NewString()
	enableNotifications(t, pool, organizationID)
	repository := NewPostgres(pool)
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	repository.Clock = func() time.Time { return now }
	intent := integrationIntent(
		organizationID,
		"notification-lease",
		"confirmation-lease",
	)
	intent.SendAt = now.Add(-time.Minute)
	if _, err := (RequestNotification{Repository: repository}).Execute(
		context.Background(),
		intent,
	); err != nil {
		t.Fatal(err)
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
	if _, err = tx.Exec(context.Background(), `
		INSERT INTO app.outbox(
			id,org_id,topic,payload,payload_hash,idempotency_key,
			request_id,actor_ref,source_version,snapshot_digest,
			correlation_id,available_at,created_at
		)
		VALUES(
			$1,$2,'BookingCreated','{}',repeat('a',64),$3,
			'request-foreign','system:scheduling',1,repeat('b',64),
			'correlation-foreign',$4,$4
		)`,
		uuid.New(),
		organizationID,
		"booking-created-foreign",
		now.Add(-time.Minute),
	); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}

	leased, err := repository.LeaseNotifications(
		context.Background(),
		10000,
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	var notificationEvent domain.OutboxEvent
	for _, event := range leased {
		if event.Topic != NotificationRequestedTopic {
			t.Fatalf("foreign context event leased: %+v", event)
		}
		if event.OrganizationID == organizationID {
			notificationEvent = event
		}
	}
	if notificationEvent.ID == "" {
		t.Fatalf("own notification was not leased: %+v", leased)
	}
	if err = repository.RetryNotification(
		context.Background(),
		notificationEvent,
	); err != nil {
		t.Fatal(err)
	}
	now = now.Add(3 * time.Second)
	leased, err = repository.LeaseNotifications(
		context.Background(),
		10000,
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	notificationEvent = domain.OutboxEvent{}
	for _, event := range leased {
		if event.Topic != NotificationRequestedTopic {
			t.Fatalf("foreign context event leased on retry: %+v", event)
		}
		if event.OrganizationID == organizationID {
			notificationEvent = event
		}
	}
	if notificationEvent.ID == "" {
		t.Fatalf("own notification was not re-leased: %+v", leased)
	}
	if err = repository.MarkNotificationPublished(
		context.Background(),
		notificationEvent,
	); err != nil {
		t.Fatal(err)
	}
	var foreignPublished bool
	if err = pool.QueryRow(context.Background(), `
		SELECT published_at IS NOT NULL
		FROM app.outbox
		WHERE org_id=$1 AND topic='BookingCreated'`,
		organizationID,
	).Scan(&foreignPublished); err != nil {
		t.Fatal(err)
	}
	if foreignPublished {
		t.Fatal("notifications relay published a foreign context event")
	}
}
