package scheduling

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	repositoryhelpers "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/repository/helpers"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	operationCreateBooking  = "scheduling.booking.create"
	operationReschedule     = "scheduling.booking.reschedule"
	operationTransition     = "scheduling.booking.transition"
	operationCreateSession  = "scheduling.session.create"
	operationCreateWaitlist = "scheduling.waitlist.create"
	operationCreateQueue    = "scheduling.queue.create"
	operationAdvanceQueue   = "scheduling.queue.advance"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool, now: time.Now}
}

func (r *PostgresRepository) ReserveBookings(
	ctx context.Context,
	metadata domain.CommandMetadata,
	series *domain.RecurrenceSeries,
	bookings []domain.Booking,
	actionTokens []domain.ActionToken,
	events []domain.Event,
) ([]domain.Booking, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, metadata.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	replayed, response, err := claimIdempotency(ctx, tx, operationCreateBooking, metadata)
	if err != nil {
		return nil, err
	}
	if replayed {
		var stored struct {
			BookingIDs []uuid.UUID `json:"booking_ids"`
		}
		if err := json.Unmarshal(response, &stored); err != nil {
			return nil, fmt.Errorf("decode booking idempotency response: %w", err)
		}
		result := make([]domain.Booking, 0, len(stored.BookingIDs))
		for _, id := range stored.BookingIDs {
			value, err := getBookingTx(ctx, tx, metadata.OrganizationID, id)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return result, nil
	}
	if series != nil {
		weekdays := make([]int16, 0, len(series.Rule.ByWeekdays))
		for _, weekday := range series.Rule.ByWeekdays {
			weekdays = append(weekdays, int16(weekday))
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO app.scheduling_recurrence_series (
				org_id,id,frequency,interval_count,occurrence_count,until_at,
				by_weekdays,timezone,status,created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			series.OrganizationID, series.ID, series.Rule.Frequency, series.Rule.Interval,
			series.Rule.Count, series.Rule.Until, weekdays, series.Timezone, series.Status, series.CreatedAt,
		)
		if err != nil {
			return nil, repositoryhelpers.MapError(err)
		}
	}
	for _, booking := range bookings {
		if err := r.insertBookingTx(ctx, tx, booking); err != nil {
			return nil, err
		}
	}
	for _, token := range actionTokens {
		if err := insertActionTokenTx(ctx, tx, token); err != nil {
			return nil, err
		}
	}
	if err := insertEvents(ctx, tx, events); err != nil {
		return nil, err
	}
	for _, booking := range bookings {
		if err := insertAudit(
			ctx, tx, metadata, "scheduling.booking.created", booking.ID.String(), nil, booking,
		); err != nil {
			return nil, err
		}
	}
	ids := make([]uuid.UUID, 0, len(bookings))
	for _, booking := range bookings {
		ids = append(ids, booking.ID)
	}
	response, _ = json.Marshal(map[string]any{"booking_ids": ids})
	if err := completeIdempotency(ctx, tx, operationCreateBooking, metadata, response); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	return bookings, nil
}

func (r *PostgresRepository) insertBookingTx(ctx context.Context, tx pgx.Tx, booking domain.Booking) error {
	if booking.SessionID != nil {
		var capacity, booked int
		var status string
		err := tx.QueryRow(ctx, `
			SELECT capacity,booked,status
			FROM app.scheduling_group_sessions
			WHERE org_id=$1 AND id=$2
			FOR UPDATE`,
			booking.OrganizationID, *booking.SessionID,
		).Scan(&capacity, &booked, &status)
		if err != nil {
			return repositoryhelpers.MapError(err)
		}
		if status != "open" || booked+booking.Participants > capacity {
			return domain.NewError(domain.CodeCapacityExceeded, "group session capacity is unavailable")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE app.scheduling_group_sessions
			SET booked=booked+$3,version=version+1,updated_at=now()
			WHERE org_id=$1 AND id=$2`,
			booking.OrganizationID, *booking.SessionID, booking.Participants,
		); err != nil {
			return repositoryhelpers.MapError(err)
		}
	} else if err := r.lockAndValidateCapacity(
		ctx, tx, booking.OrganizationID, booking.Allocations, booking.OccupiesFrom, booking.OccupiesUntil,
	); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO app.scheduling_bookings (
			org_id,id,series_id,session_id,supersedes_id,occurrence,
			branch_id,service_id,party_id,status,participants,starts_at,ends_at,
			occupies_from,occupies_until,hold_expires_at,version,
			service_name_snapshot,price_snapshot,currency_snapshot,
			duration_minutes_snapshot,timezone_snapshot,
			customer_name_snapshot,customer_email_snapshot,customer_phone_snapshot,
			notes,created_by,created_at,updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,
			$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29
		)`,
		booking.OrganizationID, booking.ID, booking.SeriesID, booking.SessionID, booking.SupersedesID,
		booking.Occurrence, booking.BranchID, booking.ServiceID, booking.PartyID, booking.Status,
		booking.Participants, booking.StartAt, booking.EndAt, booking.OccupiesFrom, booking.OccupiesUntil,
		booking.HoldExpiresAt, booking.Version, booking.ServiceName, booking.Price, booking.Currency,
		booking.DurationMinutes, booking.Timezone, booking.CustomerName, booking.CustomerEmail,
		booking.CustomerPhone, booking.Notes, booking.CreatedBy, booking.CreatedAt, booking.UpdatedAt,
	)
	if err != nil {
		return repositoryhelpers.MapError(err)
	}
	if booking.SessionID == nil {
		for _, allocation := range booking.Allocations {
			if _, err := tx.Exec(ctx, `
				INSERT INTO app.scheduling_booking_resource_allocations (
					org_id,booking_id,resource_id,allocation_mode,units,
					occupies_from,occupies_until,active
				) VALUES ($1,$2,$3,$4,$5,$6,$7,true)`,
				booking.OrganizationID, booking.ID, allocation.ResourceID, allocation.Mode,
				allocation.Units, booking.OccupiesFrom, booking.OccupiesUntil,
			); err != nil {
				return repositoryhelpers.MapError(err)
			}
		}
	} else {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.scheduling_group_participants (
				org_id,session_id,booking_id,party_id,seats,status
			) VALUES ($1,$2,$3,$4,$5,'reserved')`,
			booking.OrganizationID, *booking.SessionID, booking.ID, booking.PartyID, booking.Participants,
		); err != nil {
			return repositoryhelpers.MapError(err)
		}
	}
	if booking.Status == domain.BookingHeld {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.scheduling_holds (org_id,booking_id,expires_at)
			VALUES ($1,$2,$3)`,
			booking.OrganizationID, booking.ID, booking.HoldExpiresAt,
		); err != nil {
			return repositoryhelpers.MapError(err)
		}
	}
	return nil
}

func (r *PostgresRepository) lockAndValidateCapacity(
	ctx context.Context,
	tx pgx.Tx,
	organizationID string,
	allocations []domain.Allocation,
	startAt, endAt time.Time,
) error {
	ordered := append([]domain.Allocation(nil), allocations...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].ResourceID.String() < ordered[j].ResourceID.String()
	})
	for _, allocation := range ordered {
		var capacity int
		err := tx.QueryRow(ctx, `
			SELECT capacity FROM app.scheduling_resources
			WHERE org_id=$1 AND id=$2 AND active
			FOR UPDATE`,
			organizationID, allocation.ResourceID,
		).Scan(&capacity)
		if err != nil {
			return repositoryhelpers.MapError(err)
		}
		var allocated int
		err = tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(units),0)::integer
			FROM (
				SELECT units
				FROM app.scheduling_booking_resource_allocations
				WHERE org_id=$1 AND resource_id=$2 AND active
				  AND occupation && tstzrange($3,$4,'[)')
				UNION ALL
				SELECT units
				FROM app.scheduling_session_resource_allocations
				WHERE org_id=$1 AND resource_id=$2 AND active
				  AND occupation && tstzrange($3,$4,'[)')
			) occupied`,
			organizationID, allocation.ResourceID, startAt, endAt,
		).Scan(&allocated)
		if err != nil {
			return repositoryhelpers.MapError(err)
		}
		units := allocation.Units
		if allocation.Mode == domain.AllocationExclusive {
			units = capacity
		}
		if units <= 0 || units > capacity || allocated+units > capacity {
			code := domain.CodeCapacityExceeded
			if allocation.Mode == domain.AllocationExclusive {
				code = domain.CodeResourceConflict
			}
			return domain.NewError(code, "resource capacity is no longer available")
		}
	}
	return nil
}

func (r *PostgresRepository) RescheduleBooking(
	ctx context.Context,
	metadata domain.CommandMetadata,
	bookingID uuid.UUID,
	expectedVersion int,
	replacement domain.Booking,
	events []domain.Event,
) (domain.Booking, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, metadata.OrganizationID)
	if err != nil {
		return domain.Booking{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	replayed, response, err := claimIdempotency(ctx, tx, operationReschedule, metadata)
	if err != nil {
		return domain.Booking{}, err
	}
	if replayed {
		var stored struct {
			BookingID uuid.UUID `json:"booking_id"`
		}
		if err := json.Unmarshal(response, &stored); err != nil {
			return domain.Booking{}, err
		}
		result, err := getBookingTx(ctx, tx, metadata.OrganizationID, stored.BookingID)
		if err != nil {
			return domain.Booking{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.Booking{}, err
		}
		return result, nil
	}
	current, err := getBookingForUpdateTx(ctx, tx, metadata.OrganizationID, bookingID)
	if err != nil {
		return domain.Booking{}, err
	}
	if current.Version != expectedVersion {
		return domain.Booking{}, domain.NewError(domain.CodeBookingVersionConflict, "booking version changed")
	}
	if !current.Status.Active() || current.SessionID != nil {
		return domain.Booking{}, domain.NewError(domain.CodeBookingStateInvalid, "booking cannot be rescheduled")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE app.scheduling_booking_resource_allocations
		SET active=false
		WHERE org_id=$1 AND booking_id=$2`,
		metadata.OrganizationID, bookingID,
	); err != nil {
		return domain.Booking{}, repositoryhelpers.MapError(err)
	}
	tag, err := tx.Exec(ctx, `
		UPDATE app.scheduling_bookings
		SET status='rescheduled',version=version+1,updated_at=now()
		WHERE org_id=$1 AND id=$2 AND version=$3`,
		metadata.OrganizationID, bookingID, expectedVersion,
	)
	if err != nil {
		return domain.Booking{}, repositoryhelpers.MapError(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.Booking{}, domain.NewError(domain.CodeBookingVersionConflict, "booking version changed")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE app.scheduling_holds
		SET released_at=now(),release_reason='rescheduled'
		WHERE org_id=$1 AND booking_id=$2 AND released_at IS NULL`,
		metadata.OrganizationID, bookingID,
	); err != nil {
		return domain.Booking{}, repositoryhelpers.MapError(err)
	}
	if err := r.insertBookingTx(ctx, tx, replacement); err != nil {
		return domain.Booking{}, err
	}
	if err := insertEvents(ctx, tx, events); err != nil {
		return domain.Booking{}, err
	}
	if err := insertAudit(ctx, tx, metadata, "scheduling.booking.rescheduled", bookingID.String(), current, replacement); err != nil {
		return domain.Booking{}, err
	}
	response, _ = json.Marshal(map[string]any{"booking_id": replacement.ID})
	if err := completeIdempotency(ctx, tx, operationReschedule, metadata, response); err != nil {
		return domain.Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Booking{}, repositoryhelpers.MapError(err)
	}
	return replacement, nil
}

func (r *PostgresRepository) TransitionBooking(
	ctx context.Context,
	metadata domain.CommandMetadata,
	bookingID uuid.UUID,
	expectedVersion int,
	to domain.BookingStatus,
	events []domain.Event,
) (domain.Booking, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, metadata.OrganizationID)
	if err != nil {
		return domain.Booking{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	replayed, response, err := claimIdempotency(ctx, tx, operationTransition, metadata)
	if err != nil {
		return domain.Booking{}, err
	}
	if replayed {
		var stored struct {
			BookingID uuid.UUID `json:"booking_id"`
		}
		if err := json.Unmarshal(response, &stored); err != nil {
			return domain.Booking{}, err
		}
		result, err := getBookingTx(ctx, tx, metadata.OrganizationID, stored.BookingID)
		if err != nil {
			return domain.Booking{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.Booking{}, err
		}
		return result, nil
	}
	current, err := getBookingForUpdateTx(ctx, tx, metadata.OrganizationID, bookingID)
	if err != nil {
		return domain.Booking{}, err
	}
	if current.Version != expectedVersion {
		return domain.Booking{}, domain.NewError(domain.CodeBookingVersionConflict, "booking version changed")
	}
	if !current.Status.CanTransition(to) {
		return domain.Booking{}, domain.NewError(domain.CodeBookingStateInvalid, "booking transition is not allowed")
	}
	tag, err := tx.Exec(ctx, `
		UPDATE app.scheduling_bookings
		SET status=$4,version=version+1,updated_at=now(),
		    hold_expires_at=CASE WHEN $4='held' THEN hold_expires_at ELSE NULL END
		WHERE org_id=$1 AND id=$2 AND version=$3`,
		metadata.OrganizationID, bookingID, expectedVersion, to,
	)
	if err != nil {
		return domain.Booking{}, repositoryhelpers.MapError(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.Booking{}, domain.NewError(domain.CodeBookingVersionConflict, "booking version changed")
	}
	if !to.Active() {
		if _, err := tx.Exec(ctx, `
			UPDATE app.scheduling_booking_resource_allocations SET active=false
			WHERE org_id=$1 AND booking_id=$2`,
			metadata.OrganizationID, bookingID,
		); err != nil {
			return domain.Booking{}, repositoryhelpers.MapError(err)
		}
	}
	if to != domain.BookingHeld {
		if _, err := tx.Exec(ctx, `
			UPDATE app.scheduling_holds
			SET released_at=COALESCE(released_at,now()),release_reason=COALESCE(release_reason,$3)
			WHERE org_id=$1 AND booking_id=$2`,
			metadata.OrganizationID, bookingID, string(to),
		); err != nil {
			return domain.Booking{}, repositoryhelpers.MapError(err)
		}
	}
	if current.SessionID != nil && (to == domain.BookingCancelled || to == domain.BookingRescheduled) {
		if _, err := tx.Exec(ctx, `
			UPDATE app.scheduling_group_sessions
			SET booked=GREATEST(0,booked-$3),version=version+1,updated_at=now()
			WHERE org_id=$1 AND id=$2`,
			metadata.OrganizationID, *current.SessionID, current.Participants,
		); err != nil {
			return domain.Booking{}, repositoryhelpers.MapError(err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE app.scheduling_group_participants SET status='cancelled'
			WHERE org_id=$1 AND booking_id=$2`,
			metadata.OrganizationID, bookingID,
		); err != nil {
			return domain.Booking{}, repositoryhelpers.MapError(err)
		}
	}
	if err := insertEvents(ctx, tx, events); err != nil {
		return domain.Booking{}, err
	}
	updated, err := getBookingTx(ctx, tx, metadata.OrganizationID, bookingID)
	if err != nil {
		return domain.Booking{}, err
	}
	if err := insertAudit(ctx, tx, metadata, "scheduling.booking."+string(to), bookingID.String(), current, updated); err != nil {
		return domain.Booking{}, err
	}
	response, _ = json.Marshal(map[string]any{"booking_id": bookingID})
	if err := completeIdempotency(ctx, tx, operationTransition, metadata, response); err != nil {
		return domain.Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Booking{}, repositoryhelpers.MapError(err)
	}
	return updated, nil
}

func (r *PostgresRepository) CreateGroupSession(
	ctx context.Context,
	metadata domain.CommandMetadata,
	session domain.GroupSession,
	allocations []domain.Allocation,
	event domain.Event,
) (domain.GroupSession, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, metadata.OrganizationID)
	if err != nil {
		return domain.GroupSession{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	replayed, response, err := claimIdempotency(ctx, tx, operationCreateSession, metadata)
	if err != nil {
		return domain.GroupSession{}, err
	}
	if replayed {
		var stored struct {
			SessionID uuid.UUID `json:"session_id"`
		}
		if err := json.Unmarshal(response, &stored); err != nil {
			return domain.GroupSession{}, err
		}
		result, err := getGroupSessionTx(ctx, tx, metadata.OrganizationID, stored.SessionID, false)
		if err != nil {
			return domain.GroupSession{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.GroupSession{}, err
		}
		return result, nil
	}
	if err := r.lockAndValidateCapacity(ctx, tx, session.OrganizationID, allocations, session.StartAt, session.EndAt); err != nil {
		return domain.GroupSession{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO app.scheduling_group_sessions (
			org_id,id,branch_id,service_id,starts_at,ends_at,capacity,booked,version,status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		session.OrganizationID, session.ID, session.BranchID, session.ServiceID,
		session.StartAt, session.EndAt, session.Capacity, session.Booked, session.Version, session.Status,
	)
	if err != nil {
		return domain.GroupSession{}, repositoryhelpers.MapError(err)
	}
	for _, allocation := range allocations {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.scheduling_session_resource_allocations (
				org_id,session_id,resource_id,allocation_mode,units,
				occupies_from,occupies_until,active
			) VALUES ($1,$2,$3,$4,$5,$6,$7,true)`,
			session.OrganizationID, session.ID, allocation.ResourceID, allocation.Mode,
			allocation.Units, session.StartAt, session.EndAt,
		); err != nil {
			return domain.GroupSession{}, repositoryhelpers.MapError(err)
		}
	}
	if err := insertEvents(ctx, tx, []domain.Event{event}); err != nil {
		return domain.GroupSession{}, err
	}
	if err := insertAudit(ctx, tx, metadata, "scheduling.session.created", session.ID.String(), nil, session); err != nil {
		return domain.GroupSession{}, err
	}
	response, _ = json.Marshal(map[string]any{"session_id": session.ID})
	if err := completeIdempotency(ctx, tx, operationCreateSession, metadata, response); err != nil {
		return domain.GroupSession{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.GroupSession{}, repositoryhelpers.MapError(err)
	}
	return session, nil
}

func claimIdempotency(
	ctx context.Context,
	tx pgx.Tx,
	operation string,
	metadata domain.CommandMetadata,
) (bool, []byte, error) {
	_, err := tx.Exec(ctx, `
		INSERT INTO app.idempotency_records (
			org_id,operation,source_id,source_version,payload_hash,idempotency_key
		) VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT DO NOTHING`,
		metadata.OrganizationID, operation, metadata.SourceID, metadata.SourceVersion,
		metadata.PayloadHash, metadata.IdempotencyKey,
	)
	if err != nil {
		return false, nil, repositoryhelpers.MapError(err)
	}
	var payloadHash string
	var response []byte
	err = tx.QueryRow(ctx, `
		SELECT payload_hash,response
		FROM app.idempotency_records
		WHERE org_id=$1 AND operation=$2 AND idempotency_key=$3
		FOR UPDATE`,
		metadata.OrganizationID, operation, metadata.IdempotencyKey,
	).Scan(&payloadHash, &response)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil, domain.NewError(domain.CodeIdempotencyKeyReused, "source identity already uses another idempotency key")
	}
	if err != nil {
		return false, nil, repositoryhelpers.MapError(err)
	}
	if payloadHash != metadata.PayloadHash {
		return false, nil, domain.NewError(domain.CodeIdempotencyKeyReused, "idempotency key was reused with another payload")
	}
	return len(response) > 0, response, nil
}

func completeIdempotency(
	ctx context.Context,
	tx pgx.Tx,
	operation string,
	metadata domain.CommandMetadata,
	response []byte,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE app.idempotency_records
		SET response=$4,completed_at=now()
		WHERE org_id=$1 AND operation=$2 AND idempotency_key=$3`,
		metadata.OrganizationID, operation, metadata.IdempotencyKey, response,
	)
	return repositoryhelpers.MapError(err)
}

func insertEvents(ctx context.Context, tx pgx.Tx, events []domain.Event) error {
	for _, event := range events {
		if !integrationOutboxTopic(event.Type) {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.scheduling_events (
					org_id,id,event_type,aggregate_id,payload,payload_hash,idempotency_key,
					request_id,actor_ref,source_version,correlation_id,occurred_at
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
				ON CONFLICT (org_id,event_type,idempotency_key) DO NOTHING`,
				event.OrganizationID, event.ID, event.Type, event.AggregateID,
				json.RawMessage(event.Payload), event.PayloadHash, event.IdempotencyKey,
				event.RequestID, event.ActorID, event.SourceVersion, event.CorrelationID,
				event.AvailableAt,
			)
			if err != nil {
				return repositoryhelpers.MapError(err)
			}
			continue
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO app.outbox (
				id,org_id,topic,payload,payload_hash,idempotency_key,
				request_id,actor_ref,source_version,snapshot_digest,
				correlation_id,available_at,created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$5,$10,$11,now())
			ON CONFLICT (org_id,topic,idempotency_key) DO NOTHING`,
			event.ID, event.OrganizationID, event.Type, json.RawMessage(event.Payload), event.PayloadHash,
			event.IdempotencyKey, event.RequestID, event.ActorID, event.SourceVersion,
			event.CorrelationID, event.AvailableAt,
		)
		if err != nil {
			return repositoryhelpers.MapError(err)
		}
	}
	return nil
}

func integrationOutboxTopic(topic string) bool {
	return topic == domain.EventNotificationRequested ||
		topic == domain.EventCalendarSyncRequested
}

func insertAudit(
	ctx context.Context,
	tx pgx.Tx,
	metadata domain.CommandMetadata,
	action, aggregateID string,
	before, after any,
) error {
	beforeJSON, err := nullableJSON(before)
	if err != nil {
		return err
	}
	afterJSON, err := nullableJSON(after)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO app.scheduling_audit (
			org_id,id,actor_id,action,aggregate_id,before_state,after_state,
			request_id,correlation_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		metadata.OrganizationID, uuid.New(), metadata.ActorID, action, aggregateID,
		beforeJSON, afterJSON, metadata.RequestID, metadata.CorrelationID,
	)
	return repositoryhelpers.MapError(err)
}

func nullableJSON(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	result, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode scheduling audit: %w", err)
	}
	return result, nil
}
