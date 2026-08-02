package scheduling

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	repositoryhelpers "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/repository/helpers"
	repositorymodels "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/repository/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) SaveActionToken(ctx context.Context, value domain.ActionToken) error {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, value.OrganizationID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := insertActionTokenTx(ctx, tx, value); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertActionTokenTx(ctx context.Context, tx pgx.Tx, value domain.ActionToken) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO app.scheduling_action_tokens (
			org_id,id,booking_id,waitlist_id,result_booking_id,purpose,token_hash,
			expires_at,consumed_at,created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		value.OrganizationID, value.ID, value.BookingID, value.WaitlistID,
		value.ResultBookingID, value.Purpose, value.TokenHash, value.ExpiresAt,
		value.ConsumedAt, value.CreatedAt,
	)
	if err != nil {
		return repositoryhelpers.MapError(err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO app.scheduling_public_token_directory (token_hash,org_id)
		VALUES ($1,$2)`,
		value.TokenHash, value.OrganizationID,
	)
	return repositoryhelpers.MapError(err)
}

func (r *PostgresRepository) FindActionToken(ctx context.Context, hash string) (domain.ActionToken, error) {
	var organizationID string
	if err := r.pool.QueryRow(ctx, `
		SELECT org_id FROM app.scheduling_public_token_directory WHERE token_hash=$1`,
		hash,
	).Scan(&organizationID); err != nil {
		return domain.ActionToken{}, repositoryhelpers.MapError(err)
	}
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return domain.ActionToken{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var value domain.ActionToken
	err = tx.QueryRow(ctx, `
		SELECT org_id,id,booking_id,waitlist_id,result_booking_id,purpose,token_hash,
		       expires_at,consumed_at,created_at
		FROM app.scheduling_action_tokens
		WHERE org_id=$1 AND token_hash=$2`,
		organizationID, hash,
	).Scan(
		&value.OrganizationID, &value.ID, &value.BookingID, &value.WaitlistID,
		&value.ResultBookingID, &value.Purpose, &value.TokenHash, &value.ExpiresAt,
		&value.ConsumedAt, &value.CreatedAt,
	)
	if err != nil {
		return domain.ActionToken{}, repositoryhelpers.MapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ActionToken{}, err
	}
	return value, nil
}

func (r *PostgresRepository) ConsumeActionToken(
	ctx context.Context,
	hash string,
	consumedAt time.Time,
	resultBookingID uuid.UUID,
) error {
	token, err := r.FindActionToken(ctx, hash)
	if err != nil {
		return err
	}
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, token.OrganizationID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `
		UPDATE app.scheduling_action_tokens
		SET consumed_at=$3,result_booking_id=$4
		WHERE org_id=$1 AND token_hash=$2 AND consumed_at IS NULL AND expires_at>$3`,
		token.OrganizationID, hash, consumedAt, resultBookingID,
	)
	if err != nil {
		return repositoryhelpers.MapError(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.NewError(domain.CodeActionTokenInvalid, "action token was already consumed")
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) CreateWaitlistEntry(
	ctx context.Context,
	metadata domain.CommandMetadata,
	value domain.WaitlistEntry,
	event domain.Event,
) (domain.WaitlistEntry, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, metadata.OrganizationID)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	replayed, response, err := claimIdempotency(ctx, tx, operationCreateWaitlist, metadata)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if replayed {
		var stored repositorymodels.WaitlistIDResponse
		if err := repositoryhelpers.Decode(response, &stored); err != nil {
			return domain.WaitlistEntry{}, err
		}
		result, err := getWaitlistTx(ctx, tx, metadata.OrganizationID, stored.WaitlistID, false)
		if err != nil {
			return domain.WaitlistEntry{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.WaitlistEntry{}, err
		}
		return result, nil
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO app.scheduling_waitlist (
			org_id,id,branch_id,service_id,party_id,customer_name_snapshot,
			customer_email_snapshot,customer_phone_snapshot,preferred_from,preferred_until,
			participants,meet_requested,status,offer_expires_at,offered_starts_at,offered_ends_at,
			offered_allocations,accepted_booking_id,version,created_at,updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21
		)`,
		value.OrganizationID, value.ID, value.BranchID, value.ServiceID, value.PartyID,
		value.CustomerName, value.CustomerEmail, value.CustomerPhone,
		value.PreferredFrom, value.PreferredUntil, value.Participants, value.MeetRequested, value.Status,
		value.OfferExpiresAt, value.OfferedStartAt, value.OfferedEndAt, json.RawMessage("[]"),
		value.AcceptedBookingID, value.Version, value.CreatedAt, value.UpdatedAt,
	)
	if err != nil {
		return domain.WaitlistEntry{}, repositoryhelpers.MapError(err)
	}
	if err := insertEvents(ctx, tx, []domain.Event{event}); err != nil {
		return domain.WaitlistEntry{}, err
	}
	if err := insertAudit(ctx, tx, metadata, "scheduling.waitlist.created", value.ID.String(), nil, value); err != nil {
		return domain.WaitlistEntry{}, err
	}
	response, err = repositoryhelpers.Encode(repositorymodels.WaitlistIDResponse{WaitlistID: value.ID})
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if err := completeIdempotency(ctx, tx, operationCreateWaitlist, metadata, response); err != nil {
		return domain.WaitlistEntry{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.WaitlistEntry{}, repositoryhelpers.MapError(err)
	}
	return value, nil
}

func (r *PostgresRepository) GetWaitlist(
	ctx context.Context,
	organizationID string,
	id uuid.UUID,
) (domain.WaitlistEntry, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := getWaitlistTx(ctx, tx, organizationID, id, false)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.WaitlistEntry{}, err
	}
	return result, nil
}

func (r *PostgresRepository) ListWaitlist(
	ctx context.Context,
	organizationID string,
	branchID uuid.UUID,
) ([]domain.WaitlistEntry, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT id FROM app.scheduling_waitlist
		WHERE org_id=$1 AND branch_id=$2
		ORDER BY created_at,id`,
		organizationID, branchID,
	)
	if err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	result := make([]domain.WaitlistEntry, 0, len(ids))
	for _, id := range ids {
		value, err := getWaitlistTx(ctx, tx, organizationID, id, false)
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

func getWaitlistTx(
	ctx context.Context,
	tx pgx.Tx,
	organizationID string,
	id uuid.UUID,
	forUpdate bool,
) (domain.WaitlistEntry, error) {
	query := `
		SELECT org_id,id,branch_id,service_id,party_id,customer_name_snapshot,
		       customer_email_snapshot,customer_phone_snapshot,preferred_from,preferred_until,
		       participants,meet_requested,status,offer_expires_at,offered_starts_at,offered_ends_at,
		       offered_allocations,accepted_booking_id,version,created_at,updated_at
		FROM app.scheduling_waitlist
		WHERE org_id=$1 AND id=$2`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var value domain.WaitlistEntry
	var offeredAllocations []byte
	err := tx.QueryRow(ctx, query, organizationID, id).Scan(
		&value.OrganizationID, &value.ID, &value.BranchID, &value.ServiceID,
		&value.PartyID, &value.CustomerName, &value.CustomerEmail, &value.CustomerPhone,
		&value.PreferredFrom, &value.PreferredUntil,
		&value.Participants, &value.MeetRequested, &value.Status, &value.OfferExpiresAt,
		&value.OfferedStartAt, &value.OfferedEndAt, &offeredAllocations,
		&value.AcceptedBookingID, &value.Version, &value.CreatedAt, &value.UpdatedAt,
	)
	if err != nil {
		return domain.WaitlistEntry{}, repositoryhelpers.MapError(err)
	}
	if err := repositoryhelpers.Decode(offeredAllocations, &value.OfferedAllocations); err != nil {
		return domain.WaitlistEntry{}, err
	}
	return value, nil
}

func (r *PostgresRepository) OfferWaitlist(
	ctx context.Context,
	organizationID string,
	id uuid.UUID,
	slot domain.Slot,
	expiresAt time.Time,
	token domain.ActionToken,
	events []domain.Event,
) (domain.WaitlistEntry, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := getWaitlistTx(ctx, tx, organizationID, id, true)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if current.Status != domain.WaitlistPending {
		return current, nil
	}
	if err := insertActionTokenTx(ctx, tx, token); err != nil {
		return domain.WaitlistEntry{}, err
	}
	encodedAllocations, err := repositoryhelpers.Encode(slot.Allocations)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE app.scheduling_waitlist
		SET status='offered',offer_expires_at=$3,offered_starts_at=$4,
		    offered_ends_at=$5,offered_allocations=$6,
		    lease_token=NULL,lease_expires_at=NULL,version=version+1,updated_at=now()
		WHERE org_id=$1 AND id=$2`,
		organizationID, id, expiresAt, slot.StartAt, slot.EndAt, encodedAllocations,
	)
	if err != nil {
		return domain.WaitlistEntry{}, repositoryhelpers.MapError(err)
	}
	if err := insertEvents(ctx, tx, events); err != nil {
		return domain.WaitlistEntry{}, err
	}
	result, err := getWaitlistTx(ctx, tx, organizationID, id, false)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.WaitlistEntry{}, err
	}
	return result, nil
}

func (r *PostgresRepository) ReleaseWaitlistClaim(
	ctx context.Context,
	organizationID string,
	id uuid.UUID,
) error {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `
		UPDATE app.scheduling_waitlist
		SET lease_token=NULL,lease_expires_at=NULL
		WHERE org_id=$1 AND id=$2 AND status='pending'`,
		organizationID, id,
	)
	if err != nil {
		return repositoryhelpers.MapError(err)
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) AcceptWaitlist(
	ctx context.Context,
	organizationID string,
	id uuid.UUID,
	expectedVersion int,
	bookingID uuid.UUID,
	event domain.Event,
) (domain.WaitlistEntry, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := getWaitlistTx(ctx, tx, organizationID, id, true)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if current.Status == domain.WaitlistAccepted &&
		current.AcceptedBookingID != nil && *current.AcceptedBookingID == bookingID {
		if err := tx.Commit(ctx); err != nil {
			return domain.WaitlistEntry{}, err
		}
		return current, nil
	}
	tag, err := tx.Exec(ctx, `
		UPDATE app.scheduling_waitlist
		SET status='accepted',accepted_booking_id=$4,version=version+1,updated_at=now()
		WHERE org_id=$1 AND id=$2 AND status='offered' AND version=$3
		  AND offer_expires_at>now()`,
		organizationID, id, expectedVersion, bookingID,
	)
	if err != nil {
		return domain.WaitlistEntry{}, repositoryhelpers.MapError(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.WaitlistEntry{}, domain.NewError(domain.CodeBookingVersionConflict, "waitlist offer changed or expired")
	}
	if err := insertEvents(ctx, tx, []domain.Event{event}); err != nil {
		return domain.WaitlistEntry{}, err
	}
	result, err := getWaitlistTx(ctx, tx, organizationID, id, false)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.WaitlistEntry{}, err
	}
	return result, nil
}

func (r *PostgresRepository) CreateQueueTicket(
	ctx context.Context,
	metadata domain.CommandMetadata,
	value domain.QueueTicket,
) (domain.QueueTicket, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, metadata.OrganizationID)
	if err != nil {
		return domain.QueueTicket{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	replayed, response, err := claimIdempotency(ctx, tx, operationCreateQueue, metadata)
	if err != nil {
		return domain.QueueTicket{}, err
	}
	if replayed {
		var stored repositorymodels.TicketIDResponse
		if err := repositoryhelpers.Decode(response, &stored); err != nil {
			return domain.QueueTicket{}, err
		}
		result, err := getQueueTicketTx(ctx, tx, metadata.OrganizationID, stored.TicketID, false)
		if err != nil {
			return domain.QueueTicket{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.QueueTicket{}, err
		}
		return result, nil
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO app.scheduling_queue_counters (org_id,branch_id,service_id,next_number)
		VALUES ($1,$2,$3,2)
		ON CONFLICT (org_id,branch_id,service_id)
		DO UPDATE SET next_number=app.scheduling_queue_counters.next_number+1
		RETURNING next_number-1`,
		value.OrganizationID, value.BranchID, value.ServiceID,
	).Scan(&value.Number)
	if err != nil {
		return domain.QueueTicket{}, repositoryhelpers.MapError(err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO app.scheduling_queue_tickets (
			org_id,id,branch_id,service_id,party_id,number,priority,status,
			version,created_at,updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		value.OrganizationID, value.ID, value.BranchID, value.ServiceID, value.PartyID,
		value.Number, value.Priority, value.Status, value.Version, value.CreatedAt, value.UpdatedAt,
	)
	if err != nil {
		return domain.QueueTicket{}, repositoryhelpers.MapError(err)
	}
	response, err = repositoryhelpers.Encode(repositorymodels.TicketIDResponse{TicketID: value.ID})
	if err != nil {
		return domain.QueueTicket{}, err
	}
	if err := completeIdempotency(ctx, tx, operationCreateQueue, metadata, response); err != nil {
		return domain.QueueTicket{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.QueueTicket{}, repositoryhelpers.MapError(err)
	}
	return value, nil
}

func (r *PostgresRepository) AdvanceQueueTicket(
	ctx context.Context,
	metadata domain.CommandMetadata,
	id uuid.UUID,
	expectedVersion int,
	status domain.QueueStatus,
) (domain.QueueTicket, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, metadata.OrganizationID)
	if err != nil {
		return domain.QueueTicket{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	replayed, response, err := claimIdempotency(ctx, tx, operationAdvanceQueue, metadata)
	if err != nil {
		return domain.QueueTicket{}, err
	}
	if replayed {
		var stored repositorymodels.TicketIDResponse
		if err := repositoryhelpers.Decode(response, &stored); err != nil {
			return domain.QueueTicket{}, err
		}
		result, err := getQueueTicketTx(ctx, tx, metadata.OrganizationID, stored.TicketID, false)
		if err != nil {
			return domain.QueueTicket{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.QueueTicket{}, err
		}
		return result, nil
	}
	tag, err := tx.Exec(ctx, `
		UPDATE app.scheduling_queue_tickets
		SET status=$4,
		    called_at=CASE WHEN $4='called' THEN now() ELSE called_at END,
		    started_at=CASE WHEN $4='serving' THEN now() ELSE started_at END,
		    completed_at=CASE WHEN $4='completed' THEN now() ELSE completed_at END,
		    version=version+1,updated_at=now()
		WHERE org_id=$1 AND id=$2 AND version=$3`,
		metadata.OrganizationID, id, expectedVersion, status,
	)
	if err != nil {
		return domain.QueueTicket{}, repositoryhelpers.MapError(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.QueueTicket{}, domain.NewError(domain.CodeBookingVersionConflict, "queue ticket version changed")
	}
	response, err = repositoryhelpers.Encode(repositorymodels.TicketIDResponse{TicketID: id})
	if err != nil {
		return domain.QueueTicket{}, err
	}
	if err := completeIdempotency(ctx, tx, operationAdvanceQueue, metadata, response); err != nil {
		return domain.QueueTicket{}, err
	}
	result, err := getQueueTicketTx(ctx, tx, metadata.OrganizationID, id, false)
	if err != nil {
		return domain.QueueTicket{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.QueueTicket{}, err
	}
	return result, nil
}

func (r *PostgresRepository) ListQueue(
	ctx context.Context,
	organizationID string,
	branchID uuid.UUID,
) ([]domain.QueueTicket, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT id FROM app.scheduling_queue_tickets
		WHERE org_id=$1 AND branch_id=$2
		ORDER BY priority DESC,number`,
		organizationID, branchID,
	)
	if err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	result := make([]domain.QueueTicket, 0, len(ids))
	for _, id := range ids {
		value, err := getQueueTicketTx(ctx, tx, organizationID, id, false)
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

func getQueueTicketTx(
	ctx context.Context,
	tx pgx.Tx,
	organizationID string,
	id uuid.UUID,
	forUpdate bool,
) (domain.QueueTicket, error) {
	query := `
		SELECT org_id,id,branch_id,service_id,party_id,number,priority,status,
		       called_at,started_at,completed_at,version,created_at,updated_at
		FROM app.scheduling_queue_tickets
		WHERE org_id=$1 AND id=$2`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var value domain.QueueTicket
	err := tx.QueryRow(ctx, query, organizationID, id).Scan(
		&value.OrganizationID, &value.ID, &value.BranchID, &value.ServiceID,
		&value.PartyID, &value.Number, &value.Priority, &value.Status,
		&value.CalledAt, &value.StartedAt, &value.CompletedAt, &value.Version,
		&value.CreatedAt, &value.UpdatedAt,
	)
	return value, repositoryhelpers.MapError(err)
}

func (r *PostgresRepository) ExpireHolds(
	ctx context.Context,
	limit int,
	now time.Time,
	planner HoldExpirationPlanner,
) ([]domain.Booking, error) {
	if limit <= 0 {
		return []domain.Booking{}, nil
	}
	if planner == nil {
		return nil, domain.NewError(
			domain.CodeValidation,
			"hold expiration planner is not configured",
		)
	}
	organizations, err := r.organizationIDs(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Booking, 0, limit)
	for _, organizationID := range organizations {
		if len(result) >= limit {
			break
		}
		tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
		if err != nil {
			return nil, err
		}
		rows, err := tx.Query(ctx, `
			SELECT booking_id
			FROM app.scheduling_holds
			WHERE org_id=$1 AND released_at IS NULL AND expires_at<=$2
			ORDER BY expires_at
			FOR UPDATE SKIP LOCKED
			LIMIT $3`,
			organizationID, now, limit-len(result),
		)
		if err != nil {
			_ = tx.Rollback(ctx)
			return nil, repositoryhelpers.MapError(err)
		}
		ids := make([]uuid.UUID, 0)
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				_ = tx.Rollback(ctx)
				return nil, err
			}
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			booking, err := getBookingForUpdateTx(ctx, tx, organizationID, id)
			if err != nil {
				_ = tx.Rollback(ctx)
				return nil, err
			}
			if booking.Status != domain.BookingHeld {
				continue
			}
			_, err = tx.Exec(ctx, `
				UPDATE app.scheduling_bookings
				SET status='cancelled',hold_expires_at=NULL,version=version+1,updated_at=$3
				WHERE org_id=$1 AND id=$2`,
				organizationID, id, now,
			)
			if err == nil {
				_, err = tx.Exec(ctx, `
				UPDATE app.scheduling_booking_resource_allocations
				SET active=false WHERE org_id=$1 AND booking_id=$2`,
					organizationID, id,
				)
			}
			if err == nil {
				_, err = tx.Exec(ctx, `
				UPDATE app.scheduling_holds
				SET released_at=$3,release_reason='expired'
				WHERE org_id=$1 AND booking_id=$2`,
					organizationID, id, now,
				)
			}
			if err != nil {
				_ = tx.Rollback(ctx)
				return nil, repositoryhelpers.MapError(err)
			}
			booking.Status = domain.BookingCancelled
			booking.Version++
			booking.UpdatedAt = now
			metadata := domain.CommandMetadata{
				OrganizationID: organizationID,
				IdempotencyKey: "hold-expired:" + id.String(),
				SourceID:       id.String(),
				SourceVersion:  booking.Version,
				PayloadHash:    strings.Repeat("0", 64),
				RequestID:      "worker:holds",
				CorrelationID:  "worker:holds:" + id.String(),
				ActorID:        "system:scheduling-worker",
			}
			events := planner(metadata, booking)
			if err := insertEvents(ctx, tx, events); err != nil {
				_ = tx.Rollback(ctx)
				return nil, err
			}
			result = append(result, booking)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *PostgresRepository) ClaimReminders(
	ctx context.Context,
	limit int,
	from, until time.Time,
) ([]domain.Event, error) {
	if limit <= 0 {
		return []domain.Event{}, nil
	}
	organizations, err := r.organizationIDs(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Event, 0, limit)
	for _, organizationID := range organizations {
		if len(result) >= limit {
			break
		}
		tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
		if err != nil {
			return nil, err
		}
		rows, err := tx.Query(ctx, `
			SELECT
			  b.id,b.starts_at,b.ends_at,b.service_name_snapshot,
			  b.timezone_snapshot,b.customer_name_snapshot,
			  b.customer_phone_snapshot
			FROM app.scheduling_bookings b
			WHERE b.org_id=$1 AND b.status='confirmed'
			  AND b.starts_at>=$2 AND b.starts_at<$3
			  AND NOT EXISTS (
			    SELECT 1 FROM app.scheduling_reminders r
			    WHERE r.org_id=b.org_id AND r.booking_id=b.id AND r.reminder_at=b.starts_at
			  )
			ORDER BY b.starts_at
			FOR UPDATE SKIP LOCKED
			LIMIT $4`,
			organizationID, from, until, limit-len(result),
		)
		if err != nil {
			_ = tx.Rollback(ctx)
			return nil, repositoryhelpers.MapError(err)
		}
		values := make([]repositorymodels.ReminderDueRow, 0)
		for rows.Next() {
			var value repositorymodels.ReminderDueRow
			if err := rows.Scan(
				&value.ID,
				&value.At,
				&value.EndsAt,
				&value.ServiceName,
				&value.Timezone,
				&value.CustomerName,
				&value.CustomerPhone,
			); err != nil {
				rows.Close()
				_ = tx.Rollback(ctx)
				return nil, err
			}
			values = append(values, value)
		}
		rows.Close()
		for _, value := range values {
			payload, _ := json.Marshal(map[string]any{
				"booking_id": value.ID,
				"start_at":   value.At,
			})
			digest := sha256.Sum256(payload)
			event := domain.Event{
				ID: uuid.New(), OrganizationID: organizationID,
				Type: domain.EventReminderDue, AggregateID: value.ID.String(),
				Payload: payload, PayloadHash: hex.EncodeToString(digest[:]),
				IdempotencyKey: fmt.Sprintf("scheduling:reminder:%s:%d", value.ID, value.At.Unix()),
				RequestID:      "worker:reminders", CorrelationID: "worker:reminders:" + value.ID.String(),
				ActorID: "system:scheduling-worker", SourceVersion: 1, AvailableAt: from,
			}
			notification := newProjectionEvent(
				domain.CommandMetadata{
					OrganizationID: organizationID,
					IdempotencyKey: event.IdempotencyKey,
					SourceID:       value.ID.String(),
					SourceVersion:  1,
					PayloadHash:    event.PayloadHash,
					RequestID:      event.RequestID,
					CorrelationID:  event.CorrelationID,
					ActorID:        event.ActorID,
				},
				value.ID.String(),
				domain.EventNotificationRequested,
				domain.EventReminderDue,
				map[string]any{
					"aggregate_type": "booking",
					"aggregate_id":   value.ID,
					"booking_id":     value.ID,
					"recipient_e164": value.CustomerPhone,
					"customer_name":  value.CustomerName,
					"service_name":   value.ServiceName,
					"start_at":       value.At,
					"end_at":         value.EndsAt,
					"timezone":       value.Timezone,
				},
			)
			notification.AvailableAt = from
			if err := insertEvents(ctx, tx, []domain.Event{event, notification}); err != nil {
				_ = tx.Rollback(ctx)
				return nil, err
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO app.scheduling_reminders (
					org_id,booking_id,reminder_at,claimed_at,event_id
				) VALUES ($1,$2,$3,$4,$5)`,
				organizationID, value.ID, value.At, from, event.ID,
			)
			if err != nil {
				_ = tx.Rollback(ctx)
				return nil, repositoryhelpers.MapError(err)
			}
			result = append(result, event)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *PostgresRepository) ClaimWaitlistCandidates(
	ctx context.Context,
	limit int,
	now time.Time,
) ([]domain.WaitlistEntry, error) {
	if limit <= 0 {
		return []domain.WaitlistEntry{}, nil
	}
	organizations, err := r.organizationIDs(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.WaitlistEntry, 0, limit)
	for _, organizationID := range organizations {
		if len(result) >= limit {
			break
		}
		tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE app.scheduling_waitlist
			SET status='expired',lease_token=NULL,lease_expires_at=NULL,
			    version=version+1,updated_at=$2
			WHERE org_id=$1 AND status IN ('pending','offered')
			  AND preferred_until<=$2`,
			organizationID, now,
		); err != nil {
			_ = tx.Rollback(ctx)
			return nil, repositoryhelpers.MapError(err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE app.scheduling_waitlist
			SET status='pending',offer_expires_at=NULL,offered_starts_at=NULL,
			    offered_ends_at=NULL,offered_allocations='[]'::jsonb,
			    lease_token=NULL,lease_expires_at=NULL,version=version+1,updated_at=$2
			WHERE org_id=$1 AND status='offered' AND offer_expires_at<=$2
			  AND preferred_until>$2`,
			organizationID, now,
		); err != nil {
			_ = tx.Rollback(ctx)
			return nil, repositoryhelpers.MapError(err)
		}
		lease := uuid.New()
		rows, err := tx.Query(ctx, `
			WITH candidates AS (
			  SELECT id
			  FROM app.scheduling_waitlist
			  WHERE org_id=$1 AND status='pending'
			    AND (lease_expires_at IS NULL OR lease_expires_at<$2)
			  ORDER BY created_at
			  FOR UPDATE SKIP LOCKED
			  LIMIT $3
			)
			UPDATE app.scheduling_waitlist w
			SET lease_token=$4,lease_expires_at=$2+interval '2 minutes'
			FROM candidates c
			WHERE w.org_id=$1 AND w.id=c.id
			RETURNING w.id`,
			organizationID, now, limit-len(result), lease,
		)
		if err != nil {
			_ = tx.Rollback(ctx)
			return nil, repositoryhelpers.MapError(err)
		}
		ids := make([]uuid.UUID, 0)
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				_ = tx.Rollback(ctx)
				return nil, err
			}
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			value, err := getWaitlistTx(ctx, tx, organizationID, id, false)
			if err != nil {
				_ = tx.Rollback(ctx)
				return nil, err
			}
			result = append(result, value)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *PostgresRepository) organizationIDs(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id FROM app.organizations WHERE status='ready' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var organizationID string
		if err := rows.Scan(&organizationID); err != nil {
			return nil, err
		}
		result = append(result, organizationID)
	}
	return result, rows.Err()
}
