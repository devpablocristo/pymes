// architecture:adapter worker
package calendars

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/calendars/usecases/domain"
	workerhelpers "github.com/devpablocristo/pymes/v3/backend/internal/calendars/worker/helpers"
)

const CalendarSyncRequestedTopic = "CalendarSyncRequested"

type SyncStateStore interface {
	LeaseCalendarEvents(context.Context, int, time.Duration) ([]domain.OutboxEvent, error)
	RetryCalendarEvent(context.Context, domain.OutboxEvent) error
	DeadLetterCalendarEvent(context.Context, domain.OutboxEvent, string) error
	MarkCalendarEventPublished(context.Context, domain.OutboxEvent) error
	GetConnection(context.Context, string, string) (domain.Connection, domain.OAuthGrant, error)
	ListActiveConnections(context.Context, string) ([]domain.Connection, error)
	SaveConnectionGrant(context.Context, domain.Connection, domain.OAuthGrant) error
	MarkConnectionReauthRequired(context.Context, string, string, time.Time) error
	ListPendingConnections(context.Context, int) ([]domain.Connection, error)
	GetExternalEvent(context.Context, string, string, string) (domain.ExternalEvent, error)
	SaveExternalEvent(context.Context, domain.ExternalEvent, string, string, string, time.Time) error
	ListReconcileEvents(context.Context, int) ([]domain.ExternalEvent, error)
}

type CalendarEventProvider interface {
	Refresh(context.Context, string) (domain.OAuthGrant, error)
	CreateCalendar(context.Context, domain.OAuthGrant, string, string, string) (string, error)
	FindCalendar(context.Context, domain.OAuthGrant, string) (string, error)
	CreateEvent(context.Context, domain.OAuthGrant, string, string, string, domain.CalendarSyncCommand) (domain.ProviderEvent, error)
	GetEvent(context.Context, domain.OAuthGrant, string, string, bool) (domain.ProviderEvent, error)
	UpdateEvent(context.Context, domain.OAuthGrant, string, string, string, string, domain.CalendarSyncCommand) (domain.ProviderEvent, error)
	DeleteEvent(context.Context, domain.OAuthGrant, string, string, string) error
}

type CalendarWorker struct {
	Store       SyncStateStore
	Provider    CalendarEventProvider
	Now         func() time.Time
	LeaseFor    time.Duration
	MaxAttempts int
}

func (CalendarWorker) Topics() []string {
	return []string{CalendarSyncRequestedTopic}
}

func (worker CalendarWorker) DispatchOnce(ctx context.Context) error {
	if worker.Store == nil || worker.Provider == nil {
		return fmt.Errorf("calendar worker dependencies are not configured")
	}
	leaseFor := worker.LeaseFor
	if leaseFor <= 0 {
		leaseFor = 30 * time.Second
	}
	events, err := worker.Store.LeaseCalendarEvents(ctx, 20, leaseFor)
	if err != nil {
		return err
	}
	for _, event := range events {
		handled, deliveryErr := worker.Consume(
			ctx, event.Topic, event.OrganizationID, event.Payload,
		)
		if !handled {
			deliveryErr = fmt.Errorf("calendar worker leased unowned topic %q", event.Topic)
		}
		if deliveryErr != nil {
			maxAttempts := worker.MaxAttempts
			if maxAttempts <= 0 {
				maxAttempts = 10
			}
			if event.Attempts >= maxAttempts {
				if err := worker.Store.DeadLetterCalendarEvent(
					ctx, event, "CALENDAR_DELIVERY_FAILED",
				); err != nil {
					return err
				}
				continue
			}
			if err := worker.Store.RetryCalendarEvent(ctx, event); err != nil {
				return err
			}
			continue
		}
		if err := worker.Store.MarkCalendarEventPublished(ctx, event); err != nil {
			return err
		}
	}
	return worker.Reconcile(ctx)
}

func (worker CalendarWorker) Consume(
	ctx context.Context,
	topic, organizationID string,
	payload json.RawMessage,
) (bool, error) {
	if topic != CalendarSyncRequestedTopic {
		return false, nil
	}
	if worker.Store == nil || worker.Provider == nil {
		return true, fmt.Errorf("calendar worker dependencies are not configured")
	}
	command, err := workerhelpers.DecodeSyncRequested(
		organizationID, payload,
	)
	if err != nil {
		return true, err
	}
	connections, err := worker.Store.ListActiveConnections(
		ctx, organizationID,
	)
	if err != nil {
		return true, err
	}
	for _, connection := range connections {
		if err := worker.syncConnection(ctx, connection, command); err != nil {
			return true, err
		}
	}
	return true, nil
}

func (worker CalendarWorker) syncConnection(
	ctx context.Context,
	connection domain.Connection,
	command domain.CalendarSyncCommand,
) error {
	connection, grant, err := worker.connectionGrant(ctx, connection)
	if err != nil {
		return err
	}
	eventID := workerhelpers.EventID(
		command.OrganizationID, connection.ID, command.BookingID,
	)
	meetRequestID := ""
	if command.MeetRequested && connection.MeetEnabled {
		meetRequestID = workerhelpers.MeetRequestID(
			command.OrganizationID, connection.ID, command.BookingID,
		)
	}
	existing, err := worker.Store.GetExternalEvent(
		ctx, command.OrganizationID, connection.ID, command.BookingID,
	)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	if command.Operation == domain.SyncDelete {
		return worker.deleteEvent(
			ctx, connection, grant, command, existing, eventID,
		)
	}
	if err == nil && existing.SourceVersion > command.SourceVersion {
		return nil
	}
	if err == nil &&
		existing.SourceVersion == command.SourceVersion &&
		existing.SnapshotDigest == command.SnapshotDigest &&
		existing.Status == domain.ExternalEventSynced {
		return nil
	}
	if errors.Is(err, domain.ErrNotFound) {
		return worker.createEvent(
			ctx, connection, grant, command, eventID, meetRequestID,
		)
	}
	return worker.updateEvent(
		ctx, connection, grant, command, existing, eventID, meetRequestID,
	)
}

func (worker CalendarWorker) createEvent(
	ctx context.Context,
	connection domain.Connection,
	grant domain.OAuthGrant,
	command domain.CalendarSyncCommand,
	eventID, meetRequestID string,
) error {
	providerEvent, err := worker.Provider.CreateEvent(
		ctx, grant, connection.CalendarID, eventID, meetRequestID, command,
	)
	if errors.Is(err, domain.ErrConflict) || errors.Is(err, domain.ErrUncertain) {
		providerEvent, err = worker.Provider.GetEvent(
			ctx, grant, connection.CalendarID, eventID,
			command.MeetRequested && connection.MeetEnabled,
		)
	}
	if err != nil {
		return worker.persistFailure(
			ctx, connection, command, eventID, meetRequestID, err,
		)
	}
	if providerEvent.ID != eventID ||
		providerEvent.SnapshotDigest != command.SnapshotDigest {
		return worker.persistFailure(
			ctx, connection, command, eventID, meetRequestID,
			domain.ErrConflict,
		)
	}
	return worker.persistSuccess(
		ctx, connection, command, providerEvent, meetRequestID,
	)
}

func (worker CalendarWorker) updateEvent(
	ctx context.Context,
	connection domain.Connection,
	grant domain.OAuthGrant,
	command domain.CalendarSyncCommand,
	existing domain.ExternalEvent,
	eventID, meetRequestID string,
) error {
	providerEvent, err := worker.Provider.UpdateEvent(
		ctx, grant, connection.CalendarID, eventID, existing.ETag,
		meetRequestID, command,
	)
	if errors.Is(err, domain.ErrPreconditionFailed) ||
		errors.Is(err, domain.ErrUncertain) {
		current, getErr := worker.Provider.GetEvent(
			ctx, grant, connection.CalendarID, eventID,
			command.MeetRequested && connection.MeetEnabled,
		)
		if getErr == nil && current.SnapshotDigest == command.SnapshotDigest {
			return worker.persistSuccess(
				ctx, connection, command, current, meetRequestID,
			)
		}
		if errors.Is(err, domain.ErrPreconditionFailed) && getErr == nil {
			existing.ETag = current.ETag
			existing.Status = domain.ExternalEventReconcile
			existing.LastErrorCode = "CALENDAR_PRECONDITION_FAILED"
			existing.UpdatedAt = worker.clock()
			if saveErr := worker.Store.SaveExternalEvent(
				ctx, existing, "reconcile", "retry",
				existing.LastErrorCode, worker.clock(),
			); saveErr != nil {
				return saveErr
			}
			return domain.ErrPreconditionFailed
		}
		if getErr != nil {
			err = getErr
		}
	}
	if err != nil {
		return worker.persistFailure(
			ctx, connection, command, eventID, meetRequestID, err,
		)
	}
	return worker.persistSuccess(
		ctx, connection, command, providerEvent, meetRequestID,
	)
}

func (worker CalendarWorker) deleteEvent(
	ctx context.Context,
	connection domain.Connection,
	grant domain.OAuthGrant,
	command domain.CalendarSyncCommand,
	existing domain.ExternalEvent,
	eventID string,
) error {
	if !existing.Valid() {
		existing = domain.ExternalEvent{
			OrganizationID: command.OrganizationID,
			ConnectionID:   connection.ID, BookingID: command.BookingID,
			GoogleEventID: eventID, SourceVersion: command.SourceVersion,
			SnapshotDigest: command.SnapshotDigest,
			Status:         domain.ExternalEventDeleted,
			CreatedAt:      worker.clock(), UpdatedAt: worker.clock(),
		}
		return worker.Store.SaveExternalEvent(
			ctx, existing, "delete", "synced", "", worker.clock(),
		)
	}
	err := worker.Provider.DeleteEvent(
		ctx, grant, connection.CalendarID, eventID, existing.ETag,
	)
	if errors.Is(err, domain.ErrNotFound) {
		err = nil
	}
	if errors.Is(err, domain.ErrUncertain) {
		_, getErr := worker.Provider.GetEvent(
			ctx, grant, connection.CalendarID, eventID, false,
		)
		if errors.Is(getErr, domain.ErrNotFound) {
			err = nil
		}
	}
	if err != nil {
		return worker.persistFailure(
			ctx, connection, command, eventID,
			existing.MeetRequestID, err,
		)
	}
	existing.Status = domain.ExternalEventDeleted
	existing.SourceVersion = command.SourceVersion
	existing.SnapshotDigest = command.SnapshotDigest
	existing.LastErrorCode = ""
	existing.UpdatedAt = worker.clock()
	return worker.Store.SaveExternalEvent(
		ctx, existing, "delete", "synced", "", worker.clock(),
	)
}

func (worker CalendarWorker) connectionGrant(
	ctx context.Context,
	connection domain.Connection,
) (domain.Connection, domain.OAuthGrant, error) {
	current, grant, err := worker.Store.GetConnection(
		ctx, connection.OrganizationID, connection.ID,
	)
	if err != nil {
		return domain.Connection{}, domain.OAuthGrant{}, err
	}
	if grant.ExpiresAt.After(worker.clock().Add(2 * time.Minute)) {
		return current, grant, nil
	}
	refreshed, err := worker.Provider.Refresh(ctx, grant.RefreshToken)
	if errors.Is(err, domain.ErrReauthRequired) {
		_ = worker.Store.MarkConnectionReauthRequired(
			ctx, current.OrganizationID, current.ID, worker.clock(),
		)
		return domain.Connection{}, domain.OAuthGrant{}, err
	}
	if err != nil {
		return domain.Connection{}, domain.OAuthGrant{}, err
	}
	current.AccessTokenExpiry = refreshed.ExpiresAt
	current.Version++
	current.UpdatedAt = worker.clock()
	if err := worker.Store.SaveConnectionGrant(ctx, current, refreshed); err != nil {
		return domain.Connection{}, domain.OAuthGrant{}, err
	}
	return current, refreshed, nil
}

func (worker CalendarWorker) persistSuccess(
	ctx context.Context,
	connection domain.Connection,
	command domain.CalendarSyncCommand,
	providerEvent domain.ProviderEvent,
	meetRequestID string,
) error {
	status := domain.ExternalEventSynced
	if providerEvent.MeetStatus == "pending" {
		status = domain.ExternalEventReconcile
	}
	event := domain.ExternalEvent{
		OrganizationID: command.OrganizationID,
		ConnectionID:   connection.ID, BookingID: command.BookingID,
		GoogleEventID: providerEvent.ID, ETag: providerEvent.ETag,
		MeetRequestID: meetRequestID, MeetStatus: providerEvent.MeetStatus,
		MeetURI: providerEvent.MeetURI, SourceVersion: command.SourceVersion,
		SnapshotDigest: command.SnapshotDigest, Status: status,
		CreatedAt: worker.clock(), UpdatedAt: worker.clock(),
	}
	return worker.Store.SaveExternalEvent(
		ctx, event, string(command.Operation), "synced", "", worker.clock(),
	)
}

func (worker CalendarWorker) persistFailure(
	ctx context.Context,
	connection domain.Connection,
	command domain.CalendarSyncCommand,
	eventID, meetRequestID string,
	err error,
) error {
	code := workerhelpers.StableErrorCode(err)
	existing, existingErr := worker.Store.GetExternalEvent(
		ctx, command.OrganizationID, connection.ID, command.BookingID,
	)
	if existingErr != nil && !errors.Is(existingErr, domain.ErrNotFound) {
		return existingErr
	}
	event := domain.ExternalEvent{
		OrganizationID: command.OrganizationID,
		ConnectionID:   connection.ID, BookingID: command.BookingID,
		GoogleEventID: eventID, MeetRequestID: meetRequestID,
		SourceVersion:  command.SourceVersion,
		SnapshotDigest: command.SnapshotDigest,
		Status:         domain.ExternalEventUncertain, LastErrorCode: code,
		CreatedAt: worker.clock(), UpdatedAt: worker.clock(),
	}
	if existingErr == nil && existing.Valid() {
		event.ETag = existing.ETag
		event.MeetStatus = existing.MeetStatus
		event.MeetURI = existing.MeetURI
		if event.MeetRequestID == "" {
			event.MeetRequestID = existing.MeetRequestID
		}
		event.CreatedAt = existing.CreatedAt
	}
	if saveErr := worker.Store.SaveExternalEvent(
		ctx, event, string(command.Operation), "retry", code, worker.clock(),
	); saveErr != nil {
		return saveErr
	}
	return err
}

func (worker CalendarWorker) Reconcile(ctx context.Context) error {
	if worker.Store == nil || worker.Provider == nil {
		return fmt.Errorf("calendar worker dependencies are not configured")
	}
	if err := worker.reconcileConnections(ctx); err != nil {
		return err
	}
	events, err := worker.Store.ListReconcileEvents(ctx, 20)
	if err != nil {
		return err
	}
	for _, event := range events {
		connection, grant, err := worker.Store.GetConnection(
			ctx, event.OrganizationID, event.ConnectionID,
		)
		if err != nil {
			return err
		}
		current, err := worker.Provider.GetEvent(
			ctx, grant, connection.CalendarID, event.GoogleEventID,
			event.MeetRequestID != "",
		)
		if err != nil {
			return err
		}
		event.ETag = current.ETag
		event.MeetStatus = current.MeetStatus
		event.MeetURI = current.MeetURI
		if current.MeetStatus == "failure" {
			event.Status = domain.ExternalEventSynced
			event.LastErrorCode = "CALENDAR_MEET_FAILED"
		} else if current.MeetStatus == "pending" {
			continue
		} else {
			event.Status = domain.ExternalEventSynced
			event.LastErrorCode = ""
		}
		event.UpdatedAt = worker.clock()
		if err := worker.Store.SaveExternalEvent(
			ctx, event, "reconcile", "synced",
			event.LastErrorCode, worker.clock(),
		); err != nil {
			return err
		}
	}
	return nil
}

func (worker CalendarWorker) reconcileConnections(ctx context.Context) error {
	connections, err := worker.Store.ListPendingConnections(ctx, 20)
	if err != nil {
		return err
	}
	for _, pending := range connections {
		connection, grant, err := worker.Store.GetConnection(
			ctx, pending.OrganizationID, pending.ID,
		)
		if err != nil {
			return err
		}
		marker := "pymes-connection:" + connection.ID
		calendarID, err := worker.Provider.FindCalendar(ctx, grant, marker)
		if errors.Is(err, domain.ErrNotFound) {
			calendarID, err = worker.Provider.CreateCalendar(
				ctx, grant, "Pymes", connection.TimeZone, marker,
			)
			if errors.Is(err, domain.ErrUncertain) ||
				errors.Is(err, domain.ErrProviderUnavailable) {
				calendarID, err = worker.Provider.FindCalendar(
					ctx, grant, marker,
				)
			}
		}
		if errors.Is(err, domain.ErrNotFound) ||
			errors.Is(err, domain.ErrUncertain) ||
			errors.Is(err, domain.ErrProviderUnavailable) {
			continue
		}
		if err != nil {
			return err
		}
		connection.CalendarID = calendarID
		connection.Status = domain.ConnectionActive
		connection.Version++
		connection.UpdatedAt = worker.clock()
		if err := worker.Store.SaveConnectionGrant(
			ctx, connection, grant,
		); err != nil {
			return err
		}
	}
	return nil
}

func (worker CalendarWorker) clock() time.Time {
	if worker.Now == nil {
		return time.Now().UTC()
	}
	return worker.Now().UTC()
}
