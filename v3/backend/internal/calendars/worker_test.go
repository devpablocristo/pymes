package calendars

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/calendars/usecases/domain"
)

type calendarWorkerStore struct {
	connection domain.Connection
	grant      domain.OAuthGrant
	events     map[string]domain.ExternalEvent
	attempts   int
}

func (store *calendarWorkerStore) LeaseCalendarEvents(
	context.Context,
	int,
	time.Duration,
) ([]domain.OutboxEvent, error) {
	return nil, nil
}

func (store *calendarWorkerStore) RetryCalendarEvent(
	context.Context,
	domain.OutboxEvent,
) error {
	return nil
}

func (store *calendarWorkerStore) DeadLetterCalendarEvent(
	context.Context,
	domain.OutboxEvent,
	string,
) error {
	return nil
}

func (store *calendarWorkerStore) MarkCalendarEventPublished(
	context.Context,
	domain.OutboxEvent,
) error {
	return nil
}

func (store *calendarWorkerStore) GetConnection(
	_ context.Context,
	organizationID, connectionID string,
) (domain.Connection, domain.OAuthGrant, error) {
	if store.connection.OrganizationID != organizationID ||
		store.connection.ID != connectionID {
		return domain.Connection{}, domain.OAuthGrant{}, domain.ErrNotFound
	}
	return store.connection, store.grant, nil
}

func (store *calendarWorkerStore) ListActiveConnections(
	_ context.Context,
	organizationID string,
) ([]domain.Connection, error) {
	if store.connection.OrganizationID == organizationID &&
		store.connection.Status == domain.ConnectionActive {
		return []domain.Connection{store.connection}, nil
	}
	return nil, nil
}

func (store *calendarWorkerStore) SaveConnectionGrant(
	_ context.Context,
	connection domain.Connection,
	grant domain.OAuthGrant,
) error {
	store.connection, store.grant = connection, grant
	return nil
}

func (store *calendarWorkerStore) MarkConnectionReauthRequired(
	context.Context,
	string,
	string,
	time.Time,
) error {
	store.connection.Status = domain.ConnectionReauthRequired
	return nil
}

func (store *calendarWorkerStore) ListPendingConnections(
	context.Context,
	int,
) ([]domain.Connection, error) {
	return nil, nil
}

func (store *calendarWorkerStore) GetExternalEvent(
	_ context.Context,
	organizationID, connectionID, bookingID string,
) (domain.ExternalEvent, error) {
	event, ok := store.events[organizationID+"/"+connectionID+"/"+bookingID]
	if !ok {
		return domain.ExternalEvent{}, domain.ErrNotFound
	}
	return event, nil
}

func (store *calendarWorkerStore) SaveExternalEvent(
	_ context.Context,
	event domain.ExternalEvent,
	_, _, _ string,
	_ time.Time,
) error {
	if store.events == nil {
		store.events = make(map[string]domain.ExternalEvent)
	}
	store.events[event.OrganizationID+"/"+event.ConnectionID+"/"+event.BookingID] =
		event
	store.attempts++
	return nil
}

func (store *calendarWorkerStore) ListReconcileEvents(
	context.Context,
	int,
) ([]domain.ExternalEvent, error) {
	var result []domain.ExternalEvent
	for _, event := range store.events {
		if event.Status == domain.ExternalEventReconcile {
			result = append(result, event)
		}
	}
	return result, nil
}

type calendarWorkerGoogle struct {
	events           map[string]domain.ProviderEvent
	createCalls      int
	getCalls         int
	loseAfterPersist bool
	updateErr        error
	updateETags      []string
}

func (google *calendarWorkerGoogle) Refresh(
	context.Context,
	string,
) (domain.OAuthGrant, error) {
	return workerGrant(), nil
}

func (google *calendarWorkerGoogle) CreateCalendar(
	context.Context,
	domain.OAuthGrant,
	string,
	string,
	string,
) (string, error) {
	return "calendar", nil
}

func (google *calendarWorkerGoogle) FindCalendar(
	context.Context,
	domain.OAuthGrant,
	string,
) (string, error) {
	return "calendar", nil
}

func (google *calendarWorkerGoogle) CreateEvent(
	_ context.Context,
	_ domain.OAuthGrant,
	_, eventID, meetRequestID string,
	command domain.CalendarSyncCommand,
) (domain.ProviderEvent, error) {
	google.createCalls++
	if google.events == nil {
		google.events = make(map[string]domain.ProviderEvent)
	}
	event := domain.ProviderEvent{
		ID: eventID, ETag: `"v1"`,
		SnapshotDigest: command.SnapshotDigest,
	}
	if command.MeetRequested && meetRequestID != "" {
		event.MeetStatus = "pending"
	}
	google.events[eventID] = event
	if google.loseAfterPersist {
		google.loseAfterPersist = false
		return domain.ProviderEvent{}, domain.ErrUncertain
	}
	return event, nil
}

func (google *calendarWorkerGoogle) GetEvent(
	_ context.Context,
	_ domain.OAuthGrant,
	_, eventID string,
	_ bool,
) (domain.ProviderEvent, error) {
	google.getCalls++
	event, ok := google.events[eventID]
	if !ok {
		return domain.ProviderEvent{}, domain.ErrNotFound
	}
	return event, nil
}

func (google *calendarWorkerGoogle) UpdateEvent(
	_ context.Context,
	_ domain.OAuthGrant,
	_, eventID, etag, _ string,
	command domain.CalendarSyncCommand,
) (domain.ProviderEvent, error) {
	google.updateETags = append(google.updateETags, etag)
	if google.updateErr != nil {
		return domain.ProviderEvent{}, google.updateErr
	}
	event := google.events[eventID]
	event.ETag = `"v2"`
	event.SnapshotDigest = command.SnapshotDigest
	google.events[eventID] = event
	return event, nil
}

func TestCalendarWorkerPreservesETagWhenUpdateMustRetry(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	existing := domain.ExternalEvent{
		OrganizationID: "org", ConnectionID: "connection",
		BookingID: "booking", GoogleEventID: "event",
		ETag: `"provider-v7"`, MeetRequestID: "meet-request",
		MeetStatus: "success", MeetURI: "https://meet.test/room",
		SourceVersion: 1, SnapshotDigest: strings.Repeat("a", 64),
		Status:    domain.ExternalEventSynced,
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	store := &calendarWorkerStore{
		connection: domain.Connection{
			ID: "connection", OrganizationID: "org", ActorID: "user",
			Provider: "google", Status: domain.ConnectionActive,
			CalendarID: "calendar", TimeZone: "UTC",
			Scopes: []string{scopeCalendarCreated}, MeetEnabled: true,
			Version: 1,
		},
		grant: workerGrant(),
		events: map[string]domain.ExternalEvent{
			"org/connection/booking": existing,
		},
	}
	google := &calendarWorkerGoogle{
		updateErr: domain.ErrProviderUnavailable,
	}
	worker := CalendarWorker{
		Store: store, Provider: google, Now: func() time.Time { return now },
	}
	payload, err := json.Marshal(map[string]any{
		"command_id": "command-2", "booking_id": "booking",
		"operation": "upsert", "source_version": 2,
		"snapshot_digest": strings.Repeat("b", 64),
		"correlation_id":  "correlation-2", "summary": "Updated",
		"start":     now.Add(time.Hour).Format(time.RFC3339),
		"end":       now.Add(2 * time.Hour).Format(time.RFC3339),
		"time_zone": "UTC",
	})
	if err != nil {
		t.Fatal(err)
	}
	handled, err := worker.Consume(
		context.Background(), CalendarSyncRequestedTopic, "org", payload,
	)
	if !handled || !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	persisted := store.events["org/connection/booking"]
	if persisted.ETag != existing.ETag ||
		persisted.MeetRequestID != existing.MeetRequestID ||
		persisted.MeetURI != existing.MeetURI ||
		!persisted.CreatedAt.Equal(existing.CreatedAt) {
		t.Fatalf("retry projection lost provider state: %+v", persisted)
	}
	if len(google.updateETags) != 1 ||
		google.updateETags[0] != existing.ETag {
		t.Fatalf("update ETags=%v", google.updateETags)
	}
}

func (google *calendarWorkerGoogle) DeleteEvent(
	_ context.Context,
	_ domain.OAuthGrant,
	_, eventID, _ string,
) error {
	delete(google.events, eventID)
	return nil
}

func workerGrant() domain.OAuthGrant {
	return domain.OAuthGrant{
		AccessToken: "access", RefreshToken: "refresh",
		TokenType: "Bearer", Scope: scopeCalendarCreated,
		ExpiresAt: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
	}
}

func TestCalendarWorkerReconcilesLostCreateWithoutDuplicate(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	store := &calendarWorkerStore{
		connection: domain.Connection{
			ID: "connection", OrganizationID: "org", ActorID: "user",
			Provider: "google", Status: domain.ConnectionActive,
			CalendarID: "calendar", TimeZone: "UTC",
			Scopes: []string{scopeCalendarCreated}, MeetEnabled: true,
			Version: 1,
		},
		grant: workerGrant(), events: make(map[string]domain.ExternalEvent),
	}
	google := &calendarWorkerGoogle{loseAfterPersist: true}
	worker := CalendarWorker{
		Store: store, Provider: google, Now: func() time.Time { return now },
	}
	payload := syncPayload(t, true)
	handled, err := worker.Consume(
		context.Background(), CalendarSyncRequestedTopic, "org", payload,
	)
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if google.createCalls != 1 || google.getCalls != 1 ||
		len(store.events) != 1 {
		t.Fatalf(
			"create=%d get=%d persisted=%d",
			google.createCalls, google.getCalls, len(store.events),
		)
	}
	if _, err := worker.Consume(
		context.Background(), CalendarSyncRequestedTopic, "org", payload,
	); err != nil {
		t.Fatal(err)
	}
	if google.createCalls != 1 {
		t.Fatalf("duplicate event create calls = %d", google.createCalls)
	}
}

func TestCalendarWorkerCompletesAsynchronousMeetReconciliation(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	store := &calendarWorkerStore{
		connection: domain.Connection{
			ID: "connection", OrganizationID: "org", ActorID: "user",
			Provider: "google", Status: domain.ConnectionActive,
			CalendarID: "calendar", TimeZone: "UTC",
			Scopes: []string{scopeCalendarCreated}, MeetEnabled: true,
			Version: 1,
		},
		grant: workerGrant(), events: make(map[string]domain.ExternalEvent),
	}
	google := &calendarWorkerGoogle{}
	worker := CalendarWorker{
		Store: store, Provider: google, Now: func() time.Time { return now },
	}
	if _, err := worker.Consume(
		context.Background(), CalendarSyncRequestedTopic, "org",
		syncPayload(t, true),
	); err != nil {
		t.Fatal(err)
	}
	var persisted domain.ExternalEvent
	for _, event := range store.events {
		persisted = event
	}
	if persisted.Status != domain.ExternalEventReconcile {
		t.Fatalf("status = %q", persisted.Status)
	}
	providerEvent := google.events[persisted.GoogleEventID]
	providerEvent.MeetStatus = "success"
	providerEvent.MeetURI = "https://meet.google.com/abc-defg-hij"
	google.events[persisted.GoogleEventID] = providerEvent
	if err := worker.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	persisted = store.events["org/connection/booking"]
	if persisted.Status != domain.ExternalEventSynced ||
		persisted.MeetURI != "https://meet.google.com/abc-defg-hij" {
		t.Fatalf("reconciled = %+v", persisted)
	}
}

func TestCalendarWorkerRecordsMeetFailureWithoutBlockingBookingProjection(
	t *testing.T,
) {
	t.Parallel()
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	store := &calendarWorkerStore{
		connection: domain.Connection{
			ID: "connection", OrganizationID: "org", ActorID: "user",
			Provider: "google", Status: domain.ConnectionActive,
			CalendarID: "calendar", TimeZone: "UTC",
			Scopes: []string{scopeCalendarCreated}, MeetEnabled: true,
			Version: 1,
		},
		grant: workerGrant(), events: make(map[string]domain.ExternalEvent),
	}
	google := &calendarWorkerGoogle{}
	worker := CalendarWorker{
		Store: store, Provider: google, Now: func() time.Time { return now },
	}
	if _, err := worker.Consume(
		context.Background(), CalendarSyncRequestedTopic, "org",
		syncPayload(t, true),
	); err != nil {
		t.Fatal(err)
	}
	var persisted domain.ExternalEvent
	for _, event := range store.events {
		persisted = event
	}
	providerEvent := google.events[persisted.GoogleEventID]
	providerEvent.MeetStatus = "failure"
	google.events[persisted.GoogleEventID] = providerEvent
	if err := worker.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	persisted = store.events["org/connection/booking"]
	if persisted.Status != domain.ExternalEventSynced ||
		persisted.LastErrorCode != "CALENDAR_MEET_FAILED" {
		t.Fatalf("reconciled = %+v", persisted)
	}
}

func TestCalendarWorkerDoesNotRequestMeetWhenConnectionDisablesIt(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	store := &calendarWorkerStore{
		connection: domain.Connection{
			ID: "connection", OrganizationID: "org", ActorID: "user",
			Provider: "google", Status: domain.ConnectionActive,
			CalendarID: "calendar", TimeZone: "UTC",
			Scopes: []string{scopeCalendarCreated}, MeetEnabled: false,
			Version: 1,
		},
		grant: workerGrant(), events: make(map[string]domain.ExternalEvent),
	}
	google := &calendarWorkerGoogle{}
	worker := CalendarWorker{
		Store: store, Provider: google, Now: func() time.Time { return now },
	}
	if _, err := worker.Consume(
		context.Background(), CalendarSyncRequestedTopic, "org",
		syncPayload(t, true),
	); err != nil {
		t.Fatal(err)
	}
	persisted := store.events["org/connection/booking"]
	if persisted.MeetRequestID != "" || persisted.MeetStatus != "" ||
		persisted.MeetURI != "" ||
		persisted.Status != domain.ExternalEventSynced {
		t.Fatalf("projection unexpectedly requested Meet: %+v", persisted)
	}
}

func TestCalendarWorkerMarksRevokedTokenForReauth(t *testing.T) {
	t.Parallel()
	store := &calendarWorkerStore{
		connection: domain.Connection{
			ID: "connection", OrganizationID: "org", ActorID: "user",
			Provider: "google", Status: domain.ConnectionActive,
			CalendarID: "calendar", TimeZone: "UTC",
			Scopes: []string{scopeCalendarCreated}, Version: 1,
		},
		grant: domain.OAuthGrant{
			AccessToken: "expired", RefreshToken: "revoked",
			TokenType: "Bearer", Scope: scopeCalendarCreated,
			ExpiresAt: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
		},
		events: make(map[string]domain.ExternalEvent),
	}
	google := &calendarWorkerGoogleReauth{}
	worker := CalendarWorker{
		Store: store, Provider: google,
		Now: func() time.Time {
			return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
		},
	}
	_, err := worker.Consume(
		context.Background(), CalendarSyncRequestedTopic, "org",
		syncPayload(t, false),
	)
	if !errors.Is(err, domain.ErrReauthRequired) ||
		store.connection.Status != domain.ConnectionReauthRequired {
		t.Fatalf("err=%v connection=%+v", err, store.connection)
	}
}

type calendarWorkerGoogleReauth struct {
	calendarWorkerGoogle
}

func (*calendarWorkerGoogleReauth) Refresh(
	context.Context,
	string,
) (domain.OAuthGrant, error) {
	return domain.OAuthGrant{}, domain.ErrReauthRequired
}

func syncPayload(t *testing.T, meet bool) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"command_id": "command", "booking_id": "booking",
		"operation": "upsert", "source_version": 1,
		"snapshot_digest": strings.Repeat("a", 64),
		"correlation_id":  "correlation", "summary": "Consulta",
		"start":          "2026-08-01T10:00:00Z",
		"end":            "2026-08-01T11:00:00Z",
		"time_zone":      "America/Argentina/Buenos_Aires",
		"meet_requested": meet,
	})
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
