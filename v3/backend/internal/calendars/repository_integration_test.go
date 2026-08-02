package calendars

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/calendars/usecases/domain"
	organization "github.com/devpablocristo/pymes/v3/backend/internal/organization"
	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCalendarRepositoryEncryptsTokensAndIsolatesTenants(
	t *testing.T,
) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	cipher, err := NewLocalEnvelopeCipher(bytes.Repeat([]byte{0x31}, 32))
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool, cipher)
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	store.Now = func() time.Time { return now }
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	orgA, orgB := "calendar_a_"+suffix, "calendar_b_"+suffix
	for _, value := range []organizationdomain.Organization{
		{ID: orgA, Name: "Calendar A", Slug: "calendar-a-" + suffix, Status: organizationdomain.Ready},
		{ID: orgB, Name: "Calendar B", Slug: "calendar-b-" + suffix, Status: organizationdomain.Ready},
	} {
		if _, err := organization.New(pool).Create(ctx, value); err != nil {
			t.Fatal(err)
		}
	}
	connectionA := integrationConnection(orgA, "connection-a", now)
	connectionB := integrationConnection(orgB, "connection-b", now)
	stateA := integrationState(connectionA, strings.Repeat("a", 64), now)
	stateB := integrationState(connectionB, strings.Repeat("b", 64), now)
	if err := store.BeginOAuth(ctx, connectionA, stateA); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginOAuth(ctx, connectionB, stateB); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumeOAuthState(
		ctx, orgB, "actor-a", "session-a", stateA.Hash, now,
	); !errors.Is(err, domain.ErrOAuthStateInvalid) {
		t.Fatalf("cross-tenant OAuth state = %v", err)
	}
	grantA := domain.OAuthGrant{
		AccessToken: "access-token-a", RefreshToken: "refresh-token-a",
		TokenType: "Bearer", Scope: scopeCalendarCreated,
		ExpiresAt: now.Add(time.Hour),
	}
	connectionA.Status = domain.ConnectionActive
	connectionA.CalendarID = "google-calendar-a"
	connectionA.AccessTokenExpiry = grantA.ExpiresAt
	connectionA.UpdatedAt = now.Add(time.Minute)
	if err := store.SaveConnectionGrant(ctx, connectionA, grantA); err != nil {
		t.Fatal(err)
	}
	gotConnection, gotGrant, err := store.GetConnection(
		ctx, orgA, connectionA.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if gotConnection.OrganizationID != orgA ||
		gotGrant.AccessToken != grantA.AccessToken {
		t.Fatalf("connection=%+v grant=%+v", gotConnection, gotGrant)
	}
	if _, _, err := store.GetConnection(
		ctx, orgB, connectionA.ID,
	); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant connection = %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(
		ctx, "SELECT set_config('app.org_id',$1,true)", orgA,
	); err != nil {
		t.Fatal(err)
	}
	var envelope []byte
	if err := tx.QueryRow(ctx, `
		SELECT token_envelope FROM app.calendar_connections
		WHERE org_id=$1 AND id=$2`,
		orgA, connectionA.ID,
	).Scan(&envelope); err != nil {
		t.Fatal(err)
	}
	_ = tx.Rollback(ctx)
	if bytes.Contains(envelope, []byte(grantA.AccessToken)) ||
		bytes.Contains(envelope, []byte(grantA.RefreshToken)) {
		t.Fatal("OAuth tokens persisted in plaintext")
	}
}

func TestCalendarRelayLeasesOnlyOwnedTopic(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	cipher, _ := NewLocalEnvelopeCipher(bytes.Repeat([]byte{0x32}, 32))
	store := NewStore(pool, cipher)
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	store.Now = func() time.Time { return now }
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	orgID := "calendar_relay_" + suffix
	if _, err := organization.New(pool).Create(
		ctx,
		organizationdomain.Organization{
			ID: orgID, Name: "Calendar Relay",
			Slug:   "calendar-relay-" + suffix,
			Status: organizationdomain.Ready,
		},
	); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(
		ctx, "SELECT set_config('app.org_id',$1,true)", orgID,
	); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"booking_id": "booking"})
	for index, topic := range []string{
		CalendarSyncRequestedTopic, "NotificationRequested",
	} {
		digest := strings.Repeat(string(rune('a'+index)), 64)
		_, err = tx.Exec(ctx, `
			INSERT INTO app.outbox (
				id,org_id,topic,payload,payload_hash,idempotency_key,
				request_id,actor_ref,source_version,snapshot_digest,
				correlation_id,available_at,created_at
			) VALUES (
				gen_random_uuid(),$1,$2,$3,$4,$5,$6,'system:test',1,$7,$8,$9,$9
			)`,
			orgID, topic, payload, digest, topic+":"+suffix,
			"request:"+topic, digest, "correlation:"+topic, now,
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	events, err := store.LeaseCalendarEvents(ctx, 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Topic != CalendarSyncRequestedTopic {
		t.Fatalf("leased events = %+v", events)
	}
	if err := store.MarkCalendarEventPublished(ctx, events[0]); err != nil {
		t.Fatal(err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(
		ctx, "SELECT set_config('app.org_id',$1,true)", orgID,
	); err != nil {
		t.Fatal(err)
	}
	var notificationPublished bool
	if err := tx.QueryRow(ctx, `
		SELECT published_at IS NOT NULL FROM app.outbox
		WHERE org_id=$1 AND topic='NotificationRequested'`,
		orgID,
	).Scan(&notificationPublished); err != nil {
		t.Fatal(err)
	}
	_ = tx.Rollback(ctx)
	if notificationPublished {
		t.Fatal("calendar relay stole the notification topic")
	}
}

func integrationConnection(
	organizationID, connectionID string,
	now time.Time,
) domain.Connection {
	return domain.Connection{
		ID: connectionID, OrganizationID: organizationID,
		ActorID: "actor-" + connectionID, Provider: "google",
		Status: domain.ConnectionPending, TimeZone: "UTC",
		Scopes:  []string{scopeCalendarCreated, scopeCalendarListRead},
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
}

func integrationState(
	connection domain.Connection,
	hash string,
	now time.Time,
) domain.OAuthState {
	return domain.OAuthState{
		Hash: hash, OrganizationID: connection.OrganizationID,
		ActorID: connection.ActorID, ConnectionID: connection.ID,
		SessionBinding: "session-" + connection.ID,
		TimeZone:       connection.TimeZone, ExpiresAt: now.Add(time.Minute),
		CreatedAt: now,
	}
}
