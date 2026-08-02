package calendars

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/calendars/usecases/domain"
	workerhelpers "github.com/devpablocristo/pymes/v3/backend/internal/calendars/worker/helpers"
	workermodels "github.com/devpablocristo/pymes/v3/backend/internal/calendars/worker/models"
)

type calendarWorkerStore struct {
	connection domain.Connection
	grant      domain.OAuthGrant
	events     map[string]domain.ExternalEvent
	attempts   int
	leased     []domain.OutboxEvent
	deferred   int
	retried    int
	dead       int
	published  int
}

func (store *calendarWorkerStore) LeaseCalendarEvents(
	context.Context,
	int,
	time.Duration,
) ([]domain.OutboxEvent, error) {
	events := append([]domain.OutboxEvent(nil), store.leased...)
	store.leased = nil
	return events, nil
}

func (store *calendarWorkerStore) DeferCalendarEvent(
	context.Context,
	domain.OutboxEvent,
) error {
	store.deferred++
	return nil
}

func (store *calendarWorkerStore) RetryCalendarEvent(
	context.Context,
	domain.OutboxEvent,
) error {
	store.retried++
	return nil
}

func (store *calendarWorkerStore) DeadLetterCalendarEvent(
	context.Context,
	domain.OutboxEvent,
	string,
) error {
	store.dead++
	return nil
}

func (store *calendarWorkerStore) MarkCalendarEventPublished(
	context.Context,
	domain.OutboxEvent,
) error {
	store.published++
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
	deleteCalls      int
	refreshCalls     int
	refreshErr       error
}

func (google *calendarWorkerGoogle) Refresh(
	context.Context,
	string,
) (domain.OAuthGrant, error) {
	google.refreshCalls++
	if google.refreshErr != nil {
		return domain.OAuthGrant{}, google.refreshErr
	}
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
		Store: store, Provider: google,
		Features: calendarFeatureGate{enabled: true},
		Now:      func() time.Time { return now },
	}
	payload := syncCommandPayload(t, domain.CalendarSyncCommand{
		CommandID: "command-2", OrganizationID: "org", BookingID: "booking",
		Operation: domain.SyncUpsert, SourceVersion: 2,
		CorrelationID: "correlation-2", Summary: "Updated",
		Start: now.Add(time.Hour), End: now.Add(2 * time.Hour),
		TimeZone: "UTC",
	})
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
	google.deleteCalls++
	delete(google.events, eventID)
	return nil
}

func TestCalendarWorkerRejectsStaleAndConflictingDeletes(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	connection := domain.Connection{
		ID: "connection", OrganizationID: "org", ActorID: "user",
		Provider: "google", Status: domain.ConnectionActive,
		CalendarID: "calendar", TimeZone: "UTC",
		Scopes: []string{scopeCalendarCreated}, Version: 1,
	}
	base := domain.ExternalEvent{
		OrganizationID: "org", ConnectionID: connection.ID,
		BookingID: "booking", GoogleEventID: "event", ETag: `"v3"`,
		SourceVersion: 3, SnapshotDigest: strings.Repeat("a", 64),
		Status: domain.ExternalEventSynced, CreatedAt: now, UpdatedAt: now,
	}
	store := &calendarWorkerStore{
		connection: connection,
		grant: domain.OAuthGrant{
			AccessToken: "expired", RefreshToken: "revoked",
			TokenType: "Bearer", Scope: scopeCalendarCreated,
			ExpiresAt: now.Add(-time.Hour),
		},
		events: map[string]domain.ExternalEvent{
			"org/connection/booking": base,
		},
	}
	google := &calendarWorkerGoogle{
		refreshErr: domain.ErrReauthRequired,
		events: map[string]domain.ProviderEvent{
			"event": {ID: "event", ETag: `"v3"`},
		},
	}
	worker := CalendarWorker{
		Store: store, Provider: google,
		Features: calendarFeatureGate{enabled: true},
		Now:      func() time.Time { return now },
	}
	stale := domain.CalendarSyncCommand{
		CommandID: "delete-stale", OrganizationID: "org",
		BookingID: "booking", Operation: domain.SyncDelete,
		SourceVersion: 2, SnapshotDigest: strings.Repeat("b", 64),
		CorrelationID: "correlation-stale",
	}
	if err := worker.syncConnection(
		context.Background(),
		connection,
		stale,
	); err != nil {
		t.Fatal(err)
	}
	if google.deleteCalls != 0 ||
		google.refreshCalls != 0 ||
		store.attempts != 0 ||
		store.connection.Status != domain.ConnectionActive {
		t.Fatalf(
			"stale delete mutated projection: deletes=%d refresh=%d saves=%d status=%s",
			google.deleteCalls,
			google.refreshCalls,
			store.attempts,
			store.connection.Status,
		)
	}

	conflicting := stale
	conflicting.CommandID = "delete-conflict"
	conflicting.SourceVersion = base.SourceVersion
	if err := worker.syncConnection(
		context.Background(),
		connection,
		conflicting,
	); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("same-version conflict err=%v", err)
	}
	if google.deleteCalls != 0 || store.attempts != 0 {
		t.Fatalf(
			"conflicting delete mutated projection: deletes=%d saves=%d",
			google.deleteCalls,
			store.attempts,
		)
	}
}

func workerGrant() domain.OAuthGrant {
	return domain.OAuthGrant{
		AccessToken: "access", RefreshToken: "refresh",
		TokenType: "Bearer", Scope: scopeCalendarCreated,
		ExpiresAt: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
	}
}

func TestCalendarWorkerSkipsDisabledTenantBeforeCallingGoogle(t *testing.T) {
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
		grant:  workerGrant(),
		events: make(map[string]domain.ExternalEvent),
	}
	google := &calendarWorkerGoogle{}
	worker := CalendarWorker{
		Store: store, Provider: google,
		Features: calendarFeatureGate{enabled: false},
		Now:      func() time.Time { return now },
	}
	handled, err := worker.Consume(
		context.Background(),
		CalendarSyncRequestedTopic,
		"org",
		syncPayload(t, true),
	)
	if !errors.Is(err, domain.ErrProjectionDeferred) || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if google.createCalls != 0 ||
		len(google.updateETags) != 0 ||
		google.getCalls != 0 {
		t.Fatalf(
			"google calls create=%d update=%d get=%d",
			google.createCalls,
			len(google.updateETags),
			google.getCalls,
		)
	}
}

func TestCalendarWorkerDefersDisabledTenantWithoutConsumingAttempts(
	t *testing.T,
) {
	t.Parallel()
	payload := syncPayload(t, true)
	store := &calendarWorkerStore{
		leased: []domain.OutboxEvent{{
			ID:             "event",
			OrganizationID: "org",
			Topic:          CalendarSyncRequestedTopic,
			Payload:        payload,
			Attempts:       99,
			LeaseToken:     "lease",
		}},
	}
	worker := CalendarWorker{
		Store:       store,
		Provider:    &calendarWorkerGoogle{},
		Features:    calendarFeatureGate{enabled: false},
		MaxAttempts: 1,
	}
	if err := worker.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.deferred != 1 ||
		store.retried != 0 ||
		store.dead != 0 ||
		store.published != 0 {
		t.Fatalf(
			"deferred=%d retried=%d dead=%d published=%d",
			store.deferred,
			store.retried,
			store.dead,
			store.published,
		)
	}
}

func TestCalendarWorkerDefersUntilAnActiveConnectionExists(t *testing.T) {
	t.Parallel()
	store := &calendarWorkerStore{events: make(map[string]domain.ExternalEvent)}
	google := &calendarWorkerGoogle{}
	worker := CalendarWorker{
		Store:    store,
		Provider: google,
		Features: calendarFeatureGate{enabled: true},
	}
	handled, err := worker.Consume(
		context.Background(),
		CalendarSyncRequestedTopic,
		"org",
		syncPayload(t, true),
	)
	if !handled || !errors.Is(err, domain.ErrProjectionDeferred) {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if google.createCalls != 0 ||
		google.refreshCalls != 0 ||
		google.getCalls != 0 {
		t.Fatalf(
			"provider calls create=%d refresh=%d get=%d",
			google.createCalls,
			google.refreshCalls,
			google.getCalls,
		)
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
		Store: store, Provider: google,
		Features: calendarFeatureGate{enabled: true},
		Now:      func() time.Time { return now },
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
		Store: store, Provider: google,
		Features: calendarFeatureGate{enabled: true},
		Now:      func() time.Time { return now },
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
		Store: store, Provider: google,
		Features: calendarFeatureGate{enabled: true},
		Now:      func() time.Time { return now },
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
		Store: store, Provider: google,
		Features: calendarFeatureGate{enabled: true},
		Now:      func() time.Time { return now },
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
		Features: calendarFeatureGate{enabled: true},
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
	return syncCommandPayload(t, domain.CalendarSyncCommand{
		CommandID: "command", OrganizationID: "org", BookingID: "booking",
		Operation: domain.SyncUpsert, SourceVersion: 1,
		CorrelationID: "correlation", Summary: "Consulta",
		Start:         time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC),
		End:           time.Date(2026, 8, 1, 11, 0, 0, 0, time.UTC),
		TimeZone:      "America/Argentina/Buenos_Aires",
		MeetRequested: meet,
	})
}

func syncCommandPayload(
	t *testing.T,
	command domain.CalendarSyncCommand,
) json.RawMessage {
	t.Helper()
	command.SnapshotDigest = workerhelpers.SnapshotDigest(command)
	input := workermodels.CalendarSyncRequested{
		SchemaVersion: 1,
		CommandID:     command.CommandID, BookingID: command.BookingID,
		Operation: string(command.Operation), SourceVersion: command.SourceVersion,
		SnapshotDigest: command.SnapshotDigest,
		CorrelationID:  command.CorrelationID, Summary: command.Summary,
		Description: command.Description, Location: command.Location,
		TimeZone:       command.TimeZone,
		AttendeeEmails: append([]string(nil), command.AttendeeEmails...),
		MeetRequested:  command.MeetRequested,
	}
	if !command.Start.IsZero() {
		input.Start = command.Start.UTC().Format(time.RFC3339Nano)
	}
	if !command.End.IsZero() {
		input.End = command.End.UTC().Format(time.RFC3339Nano)
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
