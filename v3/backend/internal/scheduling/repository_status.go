package scheduling

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	repositoryhelpers "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/repository/helpers"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) ConfigureBookingStatus(
	ctx context.Context,
	metadata domain.CommandMetadata,
	configuration domain.BookingStatusConfiguration,
) (domain.BookingStatusConfiguration, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, metadata.OrganizationID)
	if err != nil {
		return domain.BookingStatusConfiguration{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	replayed, response, err := claimIdempotency(ctx, tx, operationConfigureState, metadata)
	if err != nil {
		return domain.BookingStatusConfiguration{}, err
	}
	if replayed {
		var stored domain.BookingStatusConfiguration
		if err := json.Unmarshal(response, &stored); err != nil {
			return domain.BookingStatusConfiguration{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.BookingStatusConfiguration{}, err
		}
		return stored, nil
	}

	now := r.now().UTC()
	configuration.OrganizationID = metadata.OrganizationID
	configuration.Label = strings.TrimSpace(configuration.Label)
	configuration.UpdatedAt = now
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.scheduling_booking_status_configurations (
			org_id,status,label,updated_at
		) VALUES ($1,$2,$3,$4)
		ON CONFLICT (org_id,status) DO UPDATE
		SET label=EXCLUDED.label,updated_at=EXCLUDED.updated_at`,
		configuration.OrganizationID, configuration.Status, configuration.Label, now,
	); err != nil {
		return domain.BookingStatusConfiguration{}, repositoryhelpers.MapError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE app.scheduling_booking_substates
		SET active=false,updated_at=$3
		WHERE org_id=$1 AND status=$2`,
		configuration.OrganizationID, configuration.Status, now,
	); err != nil {
		return domain.BookingStatusConfiguration{}, repositoryhelpers.MapError(err)
	}
	for index := range configuration.Substates {
		substate := &configuration.Substates[index]
		substate.Code = strings.TrimSpace(substate.Code)
		substate.Label = strings.TrimSpace(substate.Label)
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.scheduling_booking_substates (
				org_id,status,code,label,active,sort_order,updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (org_id,status,code) DO UPDATE
			SET label=EXCLUDED.label,active=EXCLUDED.active,
			    sort_order=EXCLUDED.sort_order,updated_at=EXCLUDED.updated_at`,
			configuration.OrganizationID, configuration.Status, substate.Code,
			substate.Label, substate.Active, substate.SortOrder, now,
		); err != nil {
			return domain.BookingStatusConfiguration{}, repositoryhelpers.MapError(err)
		}
	}
	if err := insertAudit(
		ctx,
		tx,
		metadata,
		"scheduling.booking_status.configured",
		string(configuration.Status),
		nil,
		configuration,
	); err != nil {
		return domain.BookingStatusConfiguration{}, err
	}
	response, err = json.Marshal(configuration)
	if err != nil {
		return domain.BookingStatusConfiguration{}, err
	}
	if err := completeIdempotency(ctx, tx, operationConfigureState, metadata, response); err != nil {
		return domain.BookingStatusConfiguration{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.BookingStatusConfiguration{}, repositoryhelpers.MapError(err)
	}
	return configuration, nil
}

func (r *PostgresRepository) ListBookingStatusConfigurations(
	ctx context.Context,
	organizationID string,
) ([]domain.BookingStatusConfiguration, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT status,label,updated_at
		FROM app.scheduling_booking_status_configurations
		WHERE org_id=$1
		ORDER BY status`,
		organizationID,
	)
	if err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	configurations := make([]domain.BookingStatusConfiguration, 0)
	for rows.Next() {
		var configuration domain.BookingStatusConfiguration
		configuration.OrganizationID = organizationID
		if err := rows.Scan(
			&configuration.Status,
			&configuration.Label,
			&configuration.UpdatedAt,
		); err != nil {
			rows.Close()
			return nil, repositoryhelpers.MapError(err)
		}
		configurations = append(configurations, configuration)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	for index := range configurations {
		substateRows, err := tx.Query(ctx, `
			SELECT code,label,active,sort_order
			FROM app.scheduling_booking_substates
			WHERE org_id=$1 AND status=$2
			ORDER BY sort_order,code`,
			organizationID, configurations[index].Status,
		)
		if err != nil {
			return nil, repositoryhelpers.MapError(err)
		}
		for substateRows.Next() {
			var substate domain.BookingSubstateDefinition
			if err := substateRows.Scan(
				&substate.Code,
				&substate.Label,
				&substate.Active,
				&substate.SortOrder,
			); err != nil {
				substateRows.Close()
				return nil, repositoryhelpers.MapError(err)
			}
			configurations[index].Substates = append(configurations[index].Substates, substate)
		}
		substateRows.Close()
		if err := substateRows.Err(); err != nil {
			return nil, repositoryhelpers.MapError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return configurations, nil
}

func (r *PostgresRepository) SetBookingSubstate(
	ctx context.Context,
	metadata domain.CommandMetadata,
	bookingID uuid.UUID,
	expectedVersion int,
	substateCode string,
) (domain.Booking, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, metadata.OrganizationID)
	if err != nil {
		return domain.Booking{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	replayed, response, err := claimIdempotency(ctx, tx, operationSetSubstate, metadata)
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
		return domain.Booking{}, domain.NewError(
			domain.CodeBookingVersionConflict,
			"booking version changed",
		)
	}
	substateCode = strings.TrimSpace(substateCode)
	if substateCode != "" {
		var active bool
		err := tx.QueryRow(ctx, `
			SELECT active
			FROM app.scheduling_booking_substates
			WHERE org_id=$1 AND status=$2 AND code=$3`,
			metadata.OrganizationID, current.Status, substateCode,
		).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !active) {
			return domain.Booking{}, domain.NewError(
				domain.CodeBookingStateInvalid,
				"booking substate is not enabled for the current internal status",
			)
		}
		if err != nil {
			return domain.Booking{}, repositoryhelpers.MapError(err)
		}
	}
	tag, err := tx.Exec(ctx, `
		UPDATE app.scheduling_bookings
		SET substate_code=$4,version=version+1,updated_at=now()
		WHERE org_id=$1 AND id=$2 AND version=$3`,
		metadata.OrganizationID,
		bookingID,
		expectedVersion,
		repositoryhelpers.NullableText(substateCode),
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
	result, err := getBookingTx(ctx, tx, metadata.OrganizationID, bookingID)
	if err != nil {
		return domain.Booking{}, err
	}
	if err := insertAudit(
		ctx,
		tx,
		metadata,
		"scheduling.booking.substate_changed",
		bookingID.String(),
		current,
		result,
	); err != nil {
		return domain.Booking{}, err
	}
	response, err = json.Marshal(map[string]any{"booking_id": bookingID})
	if err != nil {
		return domain.Booking{}, err
	}
	if err := completeIdempotency(ctx, tx, operationSetSubstate, metadata, response); err != nil {
		return domain.Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Booking{}, repositoryhelpers.MapError(err)
	}
	return result, nil
}
