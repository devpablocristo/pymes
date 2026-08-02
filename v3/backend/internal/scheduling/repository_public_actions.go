package scheduling

import (
	"context"
	"fmt"
	"time"

	repositoryhelpers "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/repository/helpers"
	repositorymodels "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/repository/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	operationPublicBookingAction  = "scheduling.public.booking_action"
	operationPublicWaitlistAction = "scheduling.public.waitlist_action"
)

// ExecutePublicBookingAction is the transaction boundary for an opaque public
// booking action. In particular, the action token is locked before the planner
// sees the booking, and token consumption is committed with the aggregate,
// events, audit trail and idempotency result.
func (r *PostgresRepository) ExecutePublicBookingAction(
	ctx context.Context,
	tokenHash string,
	purpose domain.ActionPurpose,
	now time.Time,
	metadata domain.CommandMetadata,
	planner PublicBookingActionPlanner,
) (domain.Booking, error) {
	organizationID, err := r.publicActionOrganization(ctx, tokenHash)
	if err != nil {
		return domain.Booking{}, err
	}
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return domain.Booking{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	token, err := lockPublicActionTokenTx(ctx, tx, organizationID, tokenHash)
	if err != nil {
		return domain.Booking{}, err
	}
	if token.Purpose != purpose || token.BookingID == nil || token.WaitlistID != nil {
		return domain.Booking{}, invalidPublicActionToken()
	}
	metadata.OrganizationID = organizationID
	if err := metadata.Validate(); err != nil {
		return domain.Booking{}, err
	}

	replayed, response, err := claimIdempotency(
		ctx,
		tx,
		operationPublicBookingAction,
		metadata,
	)
	if err != nil {
		return domain.Booking{}, err
	}
	if token.ConsumedAt != nil {
		if !replayed || token.ResultBookingID == nil {
			return domain.Booking{}, domain.NewError(
				domain.CodeActionTokenInvalid,
				"action token result is unavailable",
			)
		}
		var stored repositorymodels.BookingIDResponse
		if err := repositoryhelpers.Decode(response, &stored); err != nil {
			return domain.Booking{}, fmt.Errorf("decode public booking action response: %w", err)
		}
		if stored.BookingID == uuid.Nil || stored.BookingID != *token.ResultBookingID {
			return domain.Booking{}, domain.NewError(
				domain.CodeActionTokenInvalid,
				"action token result is unavailable",
			)
		}
		result, err := getBookingTx(ctx, tx, organizationID, stored.BookingID)
		if err != nil {
			return domain.Booking{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.Booking{}, repositoryhelpers.MapError(err)
		}
		return result, nil
	}
	if replayed {
		return domain.Booking{}, domain.NewError(
			domain.CodeActionTokenInvalid,
			"action token state is inconsistent",
		)
	}
	if !token.ExpiresAt.After(now) {
		return domain.Booking{}, domain.NewError(
			domain.CodeActionTokenExpired,
			"action token has expired",
		)
	}
	current, err := getBookingForUpdateTx(ctx, tx, organizationID, *token.BookingID)
	if err != nil {
		return domain.Booking{}, err
	}
	if planner == nil {
		return domain.Booking{}, invalidPublicActionToken()
	}
	plan, err := planner(metadata, token, current)
	if err != nil {
		return domain.Booking{}, err
	}
	result, err := r.applyPublicBookingActionTx(
		ctx,
		tx,
		metadata,
		token,
		current,
		plan,
	)
	if err != nil {
		return domain.Booking{}, err
	}
	response, err = repositoryhelpers.Encode(
		repositorymodels.BookingIDResponse{BookingID: result.ID},
	)
	if err != nil {
		return domain.Booking{}, err
	}
	if err := completeIdempotency(
		ctx,
		tx,
		operationPublicBookingAction,
		metadata,
		response,
	); err != nil {
		return domain.Booking{}, err
	}
	if err := consumePublicActionTokenTx(
		ctx,
		tx,
		token,
		now,
		result.ID,
	); err != nil {
		return domain.Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Booking{}, repositoryhelpers.MapError(err)
	}
	return result, nil
}

// ExecutePublicWaitlistAction accepts an offered waitlist entry and creates its
// booking in the same transaction that consumes the public token.
func (r *PostgresRepository) ExecutePublicWaitlistAction(
	ctx context.Context,
	tokenHash string,
	now time.Time,
	metadata domain.CommandMetadata,
	planner PublicWaitlistActionPlanner,
) (domain.WaitlistEntry, error) {
	organizationID, err := r.publicActionOrganization(ctx, tokenHash)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	token, err := lockPublicActionTokenTx(ctx, tx, organizationID, tokenHash)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if token.Purpose != domain.ActionAcceptWaitlist ||
		token.WaitlistID == nil ||
		token.BookingID != nil {
		return domain.WaitlistEntry{}, invalidPublicActionToken()
	}
	metadata.OrganizationID = organizationID
	if err := metadata.Validate(); err != nil {
		return domain.WaitlistEntry{}, err
	}
	replayed, response, err := claimIdempotency(
		ctx,
		tx,
		operationPublicWaitlistAction,
		metadata,
	)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if token.ConsumedAt != nil {
		if !replayed || token.ResultBookingID == nil {
			return domain.WaitlistEntry{}, domain.NewError(
				domain.CodeActionTokenInvalid,
				"action token result is unavailable",
			)
		}
		var stored repositorymodels.PublicWaitlistActionResponse
		if err := repositoryhelpers.Decode(response, &stored); err != nil {
			return domain.WaitlistEntry{}, fmt.Errorf("decode public waitlist action response: %w", err)
		}
		if stored.WaitlistID != *token.WaitlistID ||
			stored.BookingID == uuid.Nil ||
			stored.BookingID != *token.ResultBookingID {
			return domain.WaitlistEntry{}, domain.NewError(
				domain.CodeActionTokenInvalid,
				"action token result is unavailable",
			)
		}
		result, err := getWaitlistTx(
			ctx,
			tx,
			organizationID,
			stored.WaitlistID,
			false,
		)
		if err != nil {
			return domain.WaitlistEntry{}, err
		}
		if result.Status != domain.WaitlistAccepted ||
			result.AcceptedBookingID == nil ||
			*result.AcceptedBookingID != stored.BookingID {
			return domain.WaitlistEntry{}, domain.NewError(
				domain.CodeActionTokenInvalid,
				"action token result is unavailable",
			)
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.WaitlistEntry{}, repositoryhelpers.MapError(err)
		}
		return result, nil
	}
	if replayed {
		return domain.WaitlistEntry{}, domain.NewError(
			domain.CodeActionTokenInvalid,
			"action token state is inconsistent",
		)
	}
	if !token.ExpiresAt.After(now) {
		return domain.WaitlistEntry{}, domain.NewError(
			domain.CodeActionTokenExpired,
			"action token has expired",
		)
	}
	current, err := getWaitlistTx(
		ctx,
		tx,
		organizationID,
		*token.WaitlistID,
		true,
	)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if planner == nil {
		return domain.WaitlistEntry{}, invalidPublicActionToken()
	}
	plan, err := planner(metadata, token, current)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if err := validatePublicWaitlistPlan(
		organizationID,
		current,
		plan,
		now,
	); err != nil {
		return domain.WaitlistEntry{}, err
	}
	if err := r.insertBookingTx(ctx, tx, plan.Booking); err != nil {
		return domain.WaitlistEntry{}, err
	}
	for _, actionToken := range plan.ActionTokens {
		if err := insertActionTokenTx(ctx, tx, actionToken); err != nil {
			return domain.WaitlistEntry{}, err
		}
	}
	tag, err := tx.Exec(ctx, `
		UPDATE app.scheduling_waitlist
		SET status='accepted',accepted_booking_id=$4,version=version+1,
		    lease_token=NULL,lease_expires_at=NULL,updated_at=$5
		WHERE org_id=$1 AND id=$2 AND status='offered' AND version=$3
		  AND offer_expires_at>$5`,
		organizationID,
		current.ID,
		plan.ExpectedVersion,
		plan.Booking.ID,
		now,
	)
	if err != nil {
		return domain.WaitlistEntry{}, repositoryhelpers.MapError(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.WaitlistEntry{}, domain.NewError(
			domain.CodeBookingVersionConflict,
			"waitlist offer changed or expired",
		)
	}
	events := append(
		append([]domain.Event(nil), plan.BookingEvents...),
		plan.WaitlistEvent,
	)
	if err := insertEvents(ctx, tx, events); err != nil {
		return domain.WaitlistEntry{}, err
	}
	result, err := getWaitlistTx(
		ctx,
		tx,
		organizationID,
		current.ID,
		false,
	)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if err := insertAudit(
		ctx,
		tx,
		metadata,
		"scheduling.booking.created",
		plan.Booking.ID.String(),
		nil,
		plan.Booking,
	); err != nil {
		return domain.WaitlistEntry{}, err
	}
	if err := insertAudit(
		ctx,
		tx,
		metadata,
		"scheduling.waitlist.accepted",
		current.ID.String(),
		current,
		result,
	); err != nil {
		return domain.WaitlistEntry{}, err
	}
	response, err = repositoryhelpers.Encode(
		repositorymodels.PublicWaitlistActionResponse{
			WaitlistID: current.ID,
			BookingID:  plan.Booking.ID,
		},
	)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if err := completeIdempotency(
		ctx,
		tx,
		operationPublicWaitlistAction,
		metadata,
		response,
	); err != nil {
		return domain.WaitlistEntry{}, err
	}
	if err := consumePublicActionTokenTx(
		ctx,
		tx,
		token,
		now,
		plan.Booking.ID,
	); err != nil {
		return domain.WaitlistEntry{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.WaitlistEntry{}, repositoryhelpers.MapError(err)
	}
	return result, nil
}

func (r *PostgresRepository) publicActionOrganization(
	ctx context.Context,
	tokenHash string,
) (string, error) {
	var organizationID string
	err := r.pool.QueryRow(ctx, `
		SELECT org_id
		FROM app.scheduling_public_token_directory
		WHERE token_hash=$1`,
		tokenHash,
	).Scan(&organizationID)
	if err != nil || organizationID == "" {
		return "", invalidPublicActionToken()
	}
	return organizationID, nil
}

func lockPublicActionTokenTx(
	ctx context.Context,
	tx pgx.Tx,
	organizationID string,
	tokenHash string,
) (domain.ActionToken, error) {
	var value domain.ActionToken
	err := tx.QueryRow(ctx, `
		SELECT org_id,id,booking_id,waitlist_id,result_booking_id,purpose,token_hash,
		       expires_at,consumed_at,created_at
		FROM app.scheduling_action_tokens
		WHERE org_id=$1 AND token_hash=$2
		FOR UPDATE`,
		organizationID,
		tokenHash,
	).Scan(
		&value.OrganizationID,
		&value.ID,
		&value.BookingID,
		&value.WaitlistID,
		&value.ResultBookingID,
		&value.Purpose,
		&value.TokenHash,
		&value.ExpiresAt,
		&value.ConsumedAt,
		&value.CreatedAt,
	)
	if err != nil ||
		value.OrganizationID != organizationID ||
		value.TokenHash != tokenHash {
		return domain.ActionToken{}, invalidPublicActionToken()
	}
	return value, nil
}

func consumePublicActionTokenTx(
	ctx context.Context,
	tx pgx.Tx,
	token domain.ActionToken,
	consumedAt time.Time,
	resultBookingID uuid.UUID,
) error {
	tag, err := tx.Exec(ctx, `
		UPDATE app.scheduling_action_tokens
		SET consumed_at=$4,result_booking_id=$5
		WHERE org_id=$1 AND id=$2 AND token_hash=$3
		  AND consumed_at IS NULL AND expires_at>$4`,
		token.OrganizationID,
		token.ID,
		token.TokenHash,
		consumedAt,
		resultBookingID,
	)
	if err != nil {
		return repositoryhelpers.MapError(err)
	}
	if tag.RowsAffected() != 1 {
		return invalidPublicActionToken()
	}
	return nil
}

func (r *PostgresRepository) applyPublicBookingActionTx(
	ctx context.Context,
	tx pgx.Tx,
	metadata domain.CommandMetadata,
	token domain.ActionToken,
	current domain.Booking,
	plan PublicBookingActionPlan,
) (domain.Booking, error) {
	if plan.ExpectedVersion != current.Version {
		return domain.Booking{}, domain.NewError(
			domain.CodeBookingVersionConflict,
			"booking version changed",
		)
	}
	switch token.Purpose {
	case domain.ActionConfirm:
		if plan.Replacement != nil || plan.TransitionTo != domain.BookingConfirmed {
			return domain.Booking{}, invalidPublicActionToken()
		}
		return r.transitionPublicBookingTx(
			ctx,
			tx,
			metadata,
			current,
			plan,
		)
	case domain.ActionCancel:
		if plan.Replacement != nil || plan.TransitionTo != domain.BookingCancelled {
			return domain.Booking{}, invalidPublicActionToken()
		}
		return r.transitionPublicBookingTx(
			ctx,
			tx,
			metadata,
			current,
			plan,
		)
	case domain.ActionReschedule:
		if plan.Replacement == nil || plan.TransitionTo != "" {
			return domain.Booking{}, invalidPublicActionToken()
		}
		return r.reschedulePublicBookingTx(
			ctx,
			tx,
			metadata,
			current,
			plan,
		)
	default:
		return domain.Booking{}, invalidPublicActionToken()
	}
}

func (r *PostgresRepository) transitionPublicBookingTx(
	ctx context.Context,
	tx pgx.Tx,
	metadata domain.CommandMetadata,
	current domain.Booking,
	plan PublicBookingActionPlan,
) (domain.Booking, error) {
	if !current.Status.CanTransition(plan.TransitionTo) {
		return domain.Booking{}, domain.NewError(
			domain.CodeBookingStateInvalid,
			"booking transition is not allowed",
		)
	}
	tag, err := tx.Exec(ctx, `
		UPDATE app.scheduling_bookings
		SET status=$4,substate_code=NULL,version=version+1,updated_at=now(),
		    hold_expires_at=CASE WHEN $4='held' THEN hold_expires_at ELSE NULL END,
		    cancellation_reason=CASE WHEN $4='cancelled' THEN $5 ELSE cancellation_reason END
		WHERE org_id=$1 AND id=$2 AND version=$3`,
		metadata.OrganizationID,
		current.ID,
		plan.ExpectedVersion,
		plan.TransitionTo,
		plan.Reason,
	)
	if err != nil {
		return domain.Booking{}, repositoryhelpers.MapError(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.Booking{}, domain.NewError(
			domain.CodeBookingVersionConflict,
			"booking version changed",
		)
	}
	if !plan.TransitionTo.Active() {
		if _, err := tx.Exec(ctx, `
			UPDATE app.scheduling_booking_resource_allocations
			SET active=false
			WHERE org_id=$1 AND booking_id=$2`,
			metadata.OrganizationID,
			current.ID,
		); err != nil {
			return domain.Booking{}, repositoryhelpers.MapError(err)
		}
	}
	if plan.TransitionTo != domain.BookingHeld {
		if _, err := tx.Exec(ctx, `
			UPDATE app.scheduling_holds
			SET released_at=COALESCE(released_at,now()),
			    release_reason=COALESCE(release_reason,$3)
			WHERE org_id=$1 AND booking_id=$2`,
			metadata.OrganizationID,
			current.ID,
			string(plan.TransitionTo),
		); err != nil {
			return domain.Booking{}, repositoryhelpers.MapError(err)
		}
	}
	if current.SessionID != nil &&
		(plan.TransitionTo == domain.BookingCancelled ||
			plan.TransitionTo == domain.BookingRescheduled) {
		if _, err := tx.Exec(ctx, `
			UPDATE app.scheduling_group_sessions
			SET booked=GREATEST(0,booked-$3),version=version+1,updated_at=now()
			WHERE org_id=$1 AND id=$2`,
			metadata.OrganizationID,
			*current.SessionID,
			current.Participants,
		); err != nil {
			return domain.Booking{}, repositoryhelpers.MapError(err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE app.scheduling_group_participants
			SET status='cancelled'
			WHERE org_id=$1 AND booking_id=$2`,
			metadata.OrganizationID,
			current.ID,
		); err != nil {
			return domain.Booking{}, repositoryhelpers.MapError(err)
		}
	}
	if err := insertEvents(ctx, tx, plan.Events); err != nil {
		return domain.Booking{}, err
	}
	result, err := getBookingTx(
		ctx,
		tx,
		metadata.OrganizationID,
		current.ID,
	)
	if err != nil {
		return domain.Booking{}, err
	}
	if err := insertAudit(
		ctx,
		tx,
		metadata,
		"scheduling.booking."+string(plan.TransitionTo),
		current.ID.String(),
		current,
		result,
	); err != nil {
		return domain.Booking{}, err
	}
	return result, nil
}

func (r *PostgresRepository) reschedulePublicBookingTx(
	ctx context.Context,
	tx pgx.Tx,
	metadata domain.CommandMetadata,
	current domain.Booking,
	plan PublicBookingActionPlan,
) (domain.Booking, error) {
	replacement := *plan.Replacement
	if !current.Status.Active() ||
		current.SessionID != nil ||
		replacement.OrganizationID != metadata.OrganizationID ||
		replacement.ID == uuid.Nil ||
		replacement.ID == current.ID ||
		replacement.SupersedesID == nil ||
		*replacement.SupersedesID != current.ID ||
		replacement.BranchID != current.BranchID ||
		replacement.ServiceID != current.ServiceID ||
		replacement.PartyID != current.PartyID ||
		!replacement.Status.Active() ||
		replacement.Version != 1 {
		return domain.Booking{}, domain.NewError(
			domain.CodeValidation,
			"rescheduled booking is invalid",
		)
	}
	if err := replacement.Validate(); err != nil {
		return domain.Booking{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE app.scheduling_booking_resource_allocations
		SET active=false
		WHERE org_id=$1 AND booking_id=$2`,
		metadata.OrganizationID,
		current.ID,
	); err != nil {
		return domain.Booking{}, repositoryhelpers.MapError(err)
	}
	tag, err := tx.Exec(ctx, `
		UPDATE app.scheduling_bookings
		SET status='rescheduled',substate_code=NULL,version=version+1,updated_at=now()
		WHERE org_id=$1 AND id=$2 AND version=$3`,
		metadata.OrganizationID,
		current.ID,
		plan.ExpectedVersion,
	)
	if err != nil {
		return domain.Booking{}, repositoryhelpers.MapError(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.Booking{}, domain.NewError(
			domain.CodeBookingVersionConflict,
			"booking version changed",
		)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE app.scheduling_holds
		SET released_at=now(),release_reason='rescheduled'
		WHERE org_id=$1 AND booking_id=$2 AND released_at IS NULL`,
		metadata.OrganizationID,
		current.ID,
	); err != nil {
		return domain.Booking{}, repositoryhelpers.MapError(err)
	}
	if err := r.insertBookingTx(ctx, tx, replacement); err != nil {
		return domain.Booking{}, err
	}
	if err := insertEvents(ctx, tx, plan.Events); err != nil {
		return domain.Booking{}, err
	}
	if err := insertAudit(
		ctx,
		tx,
		metadata,
		"scheduling.booking.rescheduled",
		current.ID.String(),
		current,
		replacement,
	); err != nil {
		return domain.Booking{}, err
	}
	return replacement, nil
}

func validatePublicWaitlistPlan(
	organizationID string,
	current domain.WaitlistEntry,
	plan PublicWaitlistActionPlan,
	now time.Time,
) error {
	booking := plan.Booking
	if plan.ExpectedVersion != current.Version ||
		booking.OrganizationID != organizationID ||
		booking.ID == uuid.Nil ||
		booking.BranchID != current.BranchID ||
		booking.ServiceID != current.ServiceID ||
		booking.PartyID != current.PartyID ||
		booking.Status != domain.BookingConfirmed ||
		booking.Version != 1 {
		return domain.NewError(
			domain.CodeValidation,
			"waitlist acceptance plan is invalid",
		)
	}
	if err := booking.Validate(); err != nil {
		return err
	}
	for _, token := range plan.ActionTokens {
		if token.OrganizationID != organizationID ||
			token.BookingID == nil ||
			*token.BookingID != booking.ID ||
			token.WaitlistID != nil ||
			token.TokenHash == "" ||
			token.ID == uuid.Nil ||
			!token.ExpiresAt.After(now) {
			return domain.NewError(
				domain.CodeValidation,
				"waitlist booking action token is invalid",
			)
		}
	}
	if plan.WaitlistEvent.OrganizationID != organizationID ||
		plan.WaitlistEvent.AggregateID != current.ID.String() {
		return domain.NewError(
			domain.CodeValidation,
			"waitlist event is invalid",
		)
	}
	for _, event := range plan.BookingEvents {
		if event.OrganizationID != organizationID ||
			event.AggregateID != booking.ID.String() {
			return domain.NewError(
				domain.CodeValidation,
				"waitlist booking event is invalid",
			)
		}
	}
	return nil
}

func invalidPublicActionToken() error {
	return domain.NewError(
		domain.CodeActionTokenInvalid,
		"action token is invalid",
	)
}

var _ PublicActionRepository = (*PostgresRepository)(nil)
