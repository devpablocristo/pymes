package scheduling

import (
	"context"
	"fmt"
	"time"

	repositoryhelpers "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/repository/helpers"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) CreateBranch(ctx context.Context, value domain.Branch) (domain.Branch, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, value.OrganizationID)
	if err != nil {
		return domain.Branch{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	err = tx.QueryRow(ctx, `
		INSERT INTO app.scheduling_branches (
			org_id,id,code,slug,name,timezone,address,active
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING created_at,updated_at`,
		value.OrganizationID, value.ID, value.Code, value.Slug, value.Name,
		value.Timezone, value.Address, value.Active,
	).Scan(&value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return domain.Branch{}, repositoryhelpers.MapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Branch{}, err
	}
	return value, nil
}

func (r *PostgresRepository) CreateService(
	ctx context.Context,
	value domain.Service,
	requirements []domain.ResourceRequirement,
) (domain.Service, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, value.OrganizationID)
	if err != nil {
		return domain.Service{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	err = tx.QueryRow(ctx, `
		INSERT INTO app.scheduling_services (
			org_id,id,code,name,description,duration_minutes,buffer_before_minutes,
			buffer_after_minutes,slot_minutes,price,currency,fulfillment_mode,
			max_participants,allow_group,allow_waitlist,confirmation_required,active
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17
		) RETURNING created_at,updated_at`,
		value.OrganizationID, value.ID, value.Code, value.Name, value.Description,
		value.DurationMinutes, value.BufferBeforeMinutes, value.BufferAfterMinutes,
		value.SlotMinutes, value.Price, value.Currency, value.Mode, value.MaxParticipants,
		value.AllowGroup, value.AllowWaitlist, value.ConfirmationRequired, value.Active,
	).Scan(&value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return domain.Service{}, repositoryhelpers.MapError(err)
	}
	for _, requirement := range requirements {
		_, err := tx.Exec(ctx, `
			INSERT INTO app.scheduling_service_resource_requirements (
				org_id,id,service_id,resource_id,resource_kind,allocation_mode,units,optional
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			requirement.OrganizationID, requirement.ID, requirement.ServiceID,
			requirement.ResourceID, requirement.Kind, requirement.Mode, requirement.Units,
			requirement.Optional,
		)
		if err != nil {
			return domain.Service{}, repositoryhelpers.MapError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Service{}, err
	}
	return value, nil
}

func (r *PostgresRepository) CreateResource(ctx context.Context, value domain.Resource) (domain.Resource, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, value.OrganizationID)
	if err != nil {
		return domain.Resource{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	err = tx.QueryRow(ctx, `
		INSERT INTO app.scheduling_resources (
			org_id,id,branch_id,code,name,kind,capacity,timezone,active
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING created_at,updated_at`,
		value.OrganizationID, value.ID, value.BranchID, value.Code, value.Name,
		value.Kind, value.Capacity, value.Timezone, value.Active,
	).Scan(&value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return domain.Resource{}, repositoryhelpers.MapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Resource{}, err
	}
	return value, nil
}

func (r *PostgresRepository) CreateAvailabilityRule(
	ctx context.Context,
	value domain.AvailabilityRule,
) (domain.AvailabilityRule, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, value.OrganizationID)
	if err != nil {
		return domain.AvailabilityRule{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `
		INSERT INTO app.scheduling_availability_rules (
			org_id,id,branch_id,resource_id,kind,weekday,start_minute,end_minute,
			valid_from,valid_until,timezone,active
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		value.OrganizationID, value.ID, value.BranchID, value.ResourceID, value.Kind,
		int(value.Weekday), value.StartMinute, value.EndMinute, dateOnly(value.ValidFrom),
		dateOnly(value.ValidUntil), value.Timezone, value.Active,
	)
	if err != nil {
		return domain.AvailabilityRule{}, repositoryhelpers.MapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AvailabilityRule{}, err
	}
	return value, nil
}

func (r *PostgresRepository) CreateAvailabilityException(
	ctx context.Context,
	value domain.AvailabilityException,
) (domain.AvailabilityException, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, value.OrganizationID)
	if err != nil {
		return domain.AvailabilityException{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `
		INSERT INTO app.scheduling_exceptions (
			org_id,id,branch_id,resource_id,kind,starts_at,ends_at,reason
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		value.OrganizationID, value.ID, value.BranchID, value.ResourceID, value.Kind,
		value.StartAt, value.EndAt, value.Reason,
	)
	if err != nil {
		return domain.AvailabilityException{}, repositoryhelpers.MapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AvailabilityException{}, err
	}
	return value, nil
}

func (r *PostgresRepository) ListBranches(ctx context.Context, organizationID string) ([]domain.Branch, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT id FROM app.scheduling_branches WHERE org_id=$1 ORDER BY name,id`,
		organizationID,
	)
	if err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, repositoryhelpers.MapError(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	result := make([]domain.Branch, 0, len(ids))
	for _, id := range ids {
		value, err := getBranchTx(ctx, tx, organizationID, id)
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

func (r *PostgresRepository) ListServices(ctx context.Context, organizationID string) ([]domain.Service, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT id FROM app.scheduling_services WHERE org_id=$1 ORDER BY name,id`,
		organizationID,
	)
	if err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, repositoryhelpers.MapError(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	result := make([]domain.Service, 0, len(ids))
	for _, id := range ids {
		value, _, err := getServiceTx(ctx, tx, organizationID, id)
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

func (r *PostgresRepository) ListResources(
	ctx context.Context,
	organizationID string,
	branchID uuid.UUID,
) ([]domain.Resource, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT org_id,id,branch_id,code,name,kind,capacity,timezone,active,created_at,updated_at
		FROM app.scheduling_resources
		WHERE org_id=$1 AND ($2::uuid='00000000-0000-0000-0000-000000000000' OR branch_id=$2)
		ORDER BY name,id`,
		organizationID, branchID,
	)
	if err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	defer rows.Close()
	result := make([]domain.Resource, 0)
	for rows.Next() {
		var value domain.Resource
		if err := rows.Scan(
			&value.OrganizationID, &value.ID, &value.BranchID, &value.Code, &value.Name,
			&value.Kind, &value.Capacity, &value.Timezone, &value.Active,
			&value.CreatedAt, &value.UpdatedAt,
		); err != nil {
			return nil, repositoryhelpers.MapError(err)
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *PostgresRepository) ListAvailabilityRules(
	ctx context.Context,
	organizationID string,
	branchID uuid.UUID,
) ([]domain.AvailabilityRule, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT org_id,id,branch_id,resource_id,kind,weekday,start_minute,end_minute,
		       valid_from,valid_until,timezone,active
		FROM app.scheduling_availability_rules
		WHERE org_id=$1 AND branch_id=$2
		ORDER BY weekday,start_minute,id`,
		organizationID, branchID,
	)
	if err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	values := make([]domain.AvailabilityRule, 0)
	for rows.Next() {
		var value domain.AvailabilityRule
		var weekday int
		if err := rows.Scan(
			&value.OrganizationID, &value.ID, &value.BranchID, &value.ResourceID,
			&value.Kind, &weekday, &value.StartMinute, &value.EndMinute,
			&value.ValidFrom, &value.ValidUntil, &value.Timezone, &value.Active,
		); err != nil {
			rows.Close()
			return nil, repositoryhelpers.MapError(err)
		}
		value.Weekday = time.Weekday(weekday)
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, repositoryhelpers.MapError(err)
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return values, nil
}

func (r *PostgresRepository) ListAvailabilityExceptions(
	ctx context.Context,
	organizationID string,
	branchID uuid.UUID,
	from, until time.Time,
) ([]domain.AvailabilityException, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT org_id,id,branch_id,resource_id,kind,starts_at,ends_at,reason
		FROM app.scheduling_exceptions
		WHERE org_id=$1 AND branch_id=$2
		  AND tstzrange(starts_at,ends_at,'[)') && tstzrange($3,$4,'[)')
		ORDER BY starts_at,id`,
		organizationID, branchID, from, until,
	)
	if err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	defer rows.Close()
	result := make([]domain.AvailabilityException, 0)
	for rows.Next() {
		var value domain.AvailabilityException
		if err := rows.Scan(
			&value.OrganizationID, &value.ID, &value.BranchID, &value.ResourceID,
			&value.Kind, &value.StartAt, &value.EndAt, &value.Reason,
		); err != nil {
			return nil, repositoryhelpers.MapError(err)
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *PostgresRepository) GetBranch(
	ctx context.Context,
	organizationID string,
	id uuid.UUID,
) (domain.Branch, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return domain.Branch{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	value, err := getBranchTx(ctx, tx, organizationID, id)
	if err != nil {
		return domain.Branch{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Branch{}, err
	}
	return value, nil
}

func getBranchTx(ctx context.Context, tx pgx.Tx, organizationID string, id uuid.UUID) (domain.Branch, error) {
	var value domain.Branch
	err := tx.QueryRow(ctx, `
		SELECT org_id,id,code,slug,name,timezone,address,active,created_at,updated_at
		FROM app.scheduling_branches
		WHERE org_id=$1 AND id=$2`,
		organizationID, id,
	).Scan(
		&value.OrganizationID, &value.ID, &value.Code, &value.Slug, &value.Name,
		&value.Timezone, &value.Address, &value.Active, &value.CreatedAt, &value.UpdatedAt,
	)
	return value, repositoryhelpers.MapError(err)
}

func (r *PostgresRepository) GetService(
	ctx context.Context,
	organizationID string,
	id uuid.UUID,
) (domain.Service, []domain.ResourceRequirement, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return domain.Service{}, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	value, requirements, err := getServiceTx(ctx, tx, organizationID, id)
	if err != nil {
		return domain.Service{}, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Service{}, nil, err
	}
	return value, requirements, nil
}

func getServiceTx(
	ctx context.Context,
	tx pgx.Tx,
	organizationID string,
	id uuid.UUID,
) (domain.Service, []domain.ResourceRequirement, error) {
	var value domain.Service
	err := tx.QueryRow(ctx, `
		SELECT org_id,id,code,name,description,duration_minutes,buffer_before_minutes,
		       buffer_after_minutes,slot_minutes,price::text,currency,fulfillment_mode,
		       max_participants,allow_group,allow_waitlist,confirmation_required,active,
		       created_at,updated_at
		FROM app.scheduling_services
		WHERE org_id=$1 AND id=$2`,
		organizationID, id,
	).Scan(
		&value.OrganizationID, &value.ID, &value.Code, &value.Name, &value.Description,
		&value.DurationMinutes, &value.BufferBeforeMinutes, &value.BufferAfterMinutes,
		&value.SlotMinutes, &value.Price, &value.Currency, &value.Mode,
		&value.MaxParticipants, &value.AllowGroup, &value.AllowWaitlist,
		&value.ConfirmationRequired, &value.Active, &value.CreatedAt, &value.UpdatedAt,
	)
	if err != nil {
		return domain.Service{}, nil, repositoryhelpers.MapError(err)
	}
	rows, err := tx.Query(ctx, `
		SELECT org_id,id,service_id,resource_id,resource_kind,allocation_mode,units,optional
		FROM app.scheduling_service_resource_requirements
		WHERE org_id=$1 AND service_id=$2
		ORDER BY id`,
		organizationID, id,
	)
	if err != nil {
		return domain.Service{}, nil, repositoryhelpers.MapError(err)
	}
	defer rows.Close()
	requirements := make([]domain.ResourceRequirement, 0)
	for rows.Next() {
		var requirement domain.ResourceRequirement
		if err := rows.Scan(
			&requirement.OrganizationID, &requirement.ID, &requirement.ServiceID,
			&requirement.ResourceID, &requirement.Kind, &requirement.Mode,
			&requirement.Units, &requirement.Optional,
		); err != nil {
			return domain.Service{}, nil, repositoryhelpers.MapError(err)
		}
		requirements = append(requirements, requirement)
	}
	return value, requirements, repositoryhelpers.MapError(rows.Err())
}

func (r *PostgresRepository) GetResources(
	ctx context.Context,
	organizationID string,
	ids []uuid.UUID,
) ([]domain.Resource, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := getResourcesTx(ctx, tx, organizationID, ids)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func getResourcesTx(
	ctx context.Context,
	tx pgx.Tx,
	organizationID string,
	ids []uuid.UUID,
) ([]domain.Resource, error) {
	if len(ids) == 0 {
		return []domain.Resource{}, nil
	}
	rows, err := tx.Query(ctx, `
		SELECT org_id,id,branch_id,code,name,kind,capacity,timezone,active,created_at,updated_at
		FROM app.scheduling_resources
		WHERE org_id=$1 AND id=ANY($2::uuid[])
		ORDER BY id`,
		organizationID, ids,
	)
	if err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	defer rows.Close()
	result := make([]domain.Resource, 0, len(ids))
	for rows.Next() {
		var value domain.Resource
		if err := rows.Scan(
			&value.OrganizationID, &value.ID, &value.BranchID, &value.Code, &value.Name,
			&value.Kind, &value.Capacity, &value.Timezone, &value.Active,
			&value.CreatedAt, &value.UpdatedAt,
		); err != nil {
			return nil, repositoryhelpers.MapError(err)
		}
		result = append(result, value)
	}
	return result, repositoryhelpers.MapError(rows.Err())
}

func (r *PostgresRepository) LoadAvailability(
	ctx context.Context,
	query domain.AvailabilityQuery,
) (domain.AvailabilitySnapshot, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, query.OrganizationID)
	if err != nil {
		return domain.AvailabilitySnapshot{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	branch, err := getBranchTx(ctx, tx, query.OrganizationID, query.BranchID)
	if err != nil {
		return domain.AvailabilitySnapshot{}, err
	}
	service, _, err := getServiceTx(ctx, tx, query.OrganizationID, query.ServiceID)
	if err != nil {
		return domain.AvailabilitySnapshot{}, err
	}
	ids := allocationIDs(query.Allocations)
	resources, err := getResourcesTx(ctx, tx, query.OrganizationID, ids)
	if err != nil {
		return domain.AvailabilitySnapshot{}, err
	}
	rules, err := loadAvailabilityRulesTx(ctx, tx, query.OrganizationID, query.BranchID, ids)
	if err != nil {
		return domain.AvailabilitySnapshot{}, err
	}
	expandedQuery := query
	expandedQuery.From = query.From.Add(-time.Duration(service.BufferBeforeMinutes) * time.Minute)
	expandedQuery.Until = query.Until.Add(time.Duration(service.BufferAfterMinutes) * time.Minute)
	exceptions, err := loadExceptionsTx(ctx, tx, expandedQuery, ids)
	if err != nil {
		return domain.AvailabilitySnapshot{}, err
	}
	occupancies, err := loadOccupanciesTx(ctx, tx, expandedQuery, ids)
	if err != nil {
		return domain.AvailabilitySnapshot{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AvailabilitySnapshot{}, err
	}
	return domain.AvailabilitySnapshot{
		Branch: branch, Service: service, Resources: resources,
		Rules: rules, Exceptions: exceptions, Occupancies: occupancies,
	}, nil
}

func loadAvailabilityRulesTx(
	ctx context.Context,
	tx pgx.Tx,
	organizationID string,
	branchID uuid.UUID,
	resourceIDs []uuid.UUID,
) ([]domain.AvailabilityRule, error) {
	rows, err := tx.Query(ctx, `
		SELECT org_id,id,branch_id,resource_id,kind,weekday,start_minute,end_minute,
		       valid_from,valid_until,timezone,active
		FROM app.scheduling_availability_rules
		WHERE org_id=$1 AND branch_id=$2
		  AND (resource_id IS NULL OR resource_id=ANY($3::uuid[]))
		ORDER BY weekday,start_minute,id`,
		organizationID, branchID, resourceIDs,
	)
	if err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	defer rows.Close()
	result := make([]domain.AvailabilityRule, 0)
	for rows.Next() {
		var value domain.AvailabilityRule
		var weekday int
		if err := rows.Scan(
			&value.OrganizationID, &value.ID, &value.BranchID, &value.ResourceID,
			&value.Kind, &weekday, &value.StartMinute, &value.EndMinute,
			&value.ValidFrom, &value.ValidUntil, &value.Timezone, &value.Active,
		); err != nil {
			return nil, repositoryhelpers.MapError(err)
		}
		value.Weekday = time.Weekday(weekday)
		result = append(result, value)
	}
	return result, repositoryhelpers.MapError(rows.Err())
}

func loadExceptionsTx(
	ctx context.Context,
	tx pgx.Tx,
	query domain.AvailabilityQuery,
	resourceIDs []uuid.UUID,
) ([]domain.AvailabilityException, error) {
	rows, err := tx.Query(ctx, `
		SELECT org_id,id,branch_id,resource_id,kind,starts_at,ends_at,reason
		FROM app.scheduling_exceptions
		WHERE org_id=$1 AND branch_id=$2
		  AND (resource_id IS NULL OR resource_id=ANY($3::uuid[]))
		  AND tstzrange(starts_at,ends_at,'[)') && tstzrange($4,$5,'[)')
		ORDER BY starts_at,id`,
		query.OrganizationID, query.BranchID, resourceIDs, query.From, query.Until,
	)
	if err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	defer rows.Close()
	result := make([]domain.AvailabilityException, 0)
	for rows.Next() {
		var value domain.AvailabilityException
		if err := rows.Scan(
			&value.OrganizationID, &value.ID, &value.BranchID, &value.ResourceID,
			&value.Kind, &value.StartAt, &value.EndAt, &value.Reason,
		); err != nil {
			return nil, repositoryhelpers.MapError(err)
		}
		result = append(result, value)
	}
	return result, repositoryhelpers.MapError(rows.Err())
}

func loadOccupanciesTx(
	ctx context.Context,
	tx pgx.Tx,
	query domain.AvailabilityQuery,
	resourceIDs []uuid.UUID,
) ([]domain.Occupancy, error) {
	rows, err := tx.Query(ctx, `
		SELECT resource_id,occupies_from,occupies_until,units,service_id,aggregate_id,booking_open
		FROM (
			SELECT a.resource_id,a.occupies_from,a.occupies_until,a.units,b.service_id,
			       b.id AS aggregate_id,
			       (a.active AND b.status IN ('held','pending_confirmation','confirmed','checked_in')
			        AND (b.status <> 'held' OR b.hold_expires_at > now())) AS booking_open
			FROM app.scheduling_booking_resource_allocations a
			JOIN app.scheduling_bookings b
			  ON b.org_id=a.org_id AND b.id=a.booking_id
			WHERE a.org_id=$1 AND a.resource_id=ANY($2::uuid[])
			  AND a.occupation && tstzrange($3,$4,'[)')
			UNION ALL
			SELECT a.resource_id,a.occupies_from,a.occupies_until,a.units,s.service_id,
			       s.id AS aggregate_id,
			       (a.active AND s.status='open') AS booking_open
			FROM app.scheduling_session_resource_allocations a
			JOIN app.scheduling_group_sessions s
			  ON s.org_id=a.org_id AND s.id=a.session_id
			WHERE a.org_id=$1 AND a.resource_id=ANY($2::uuid[])
			  AND a.occupation && tstzrange($3,$4,'[)')
		) occupied
		ORDER BY occupies_from,resource_id`,
		query.OrganizationID, resourceIDs, query.From, query.Until,
	)
	if err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	defer rows.Close()
	result := make([]domain.Occupancy, 0)
	for rows.Next() {
		var value domain.Occupancy
		if err := rows.Scan(
			&value.ResourceID, &value.StartAt, &value.EndAt, &value.Units,
			&value.ServiceID, &value.BookingID, &value.BookingOpen,
		); err != nil {
			return nil, repositoryhelpers.MapError(err)
		}
		result = append(result, value)
	}
	return result, repositoryhelpers.MapError(rows.Err())
}

func (r *PostgresRepository) GetBooking(
	ctx context.Context,
	organizationID string,
	id uuid.UUID,
) (domain.Booking, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return domain.Booking{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := getBookingTx(ctx, tx, organizationID, id)
	if err != nil {
		return domain.Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Booking{}, err
	}
	return result, nil
}

func getBookingTx(
	ctx context.Context,
	tx pgx.Tx,
	organizationID string,
	id uuid.UUID,
) (domain.Booking, error) {
	return scanBooking(ctx, tx, organizationID, id, false)
}

func getBookingForUpdateTx(
	ctx context.Context,
	tx pgx.Tx,
	organizationID string,
	id uuid.UUID,
) (domain.Booking, error) {
	return scanBooking(ctx, tx, organizationID, id, true)
}

func scanBooking(
	ctx context.Context,
	tx pgx.Tx,
	organizationID string,
	id uuid.UUID,
	forUpdate bool,
) (domain.Booking, error) {
	query := `
		SELECT org_id,id,series_id,session_id,supersedes_id,occurrence,
		       branch_id,service_id,party_id,status,COALESCE(substate_code,''),participants,starts_at,ends_at,
		       occupies_from,occupies_until,hold_expires_at,version,
		       service_name_snapshot,price_snapshot::text,currency_snapshot,
			       duration_minutes_snapshot,timezone_snapshot,
			       customer_name_snapshot,customer_email_snapshot,customer_phone_snapshot,
			       meet_requested,notes,cancellation_reason,created_by,created_at,updated_at
		FROM app.scheduling_bookings
		WHERE org_id=$1 AND id=$2`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var value domain.Booking
	err := tx.QueryRow(ctx, query, organizationID, id).Scan(
		&value.OrganizationID, &value.ID, &value.SeriesID, &value.SessionID,
		&value.SupersedesID, &value.Occurrence, &value.BranchID, &value.ServiceID,
		&value.PartyID, &value.Status, &value.SubstateCode, &value.Participants,
		&value.StartAt, &value.EndAt,
		&value.OccupiesFrom, &value.OccupiesUntil, &value.HoldExpiresAt, &value.Version,
		&value.ServiceName, &value.Price, &value.Currency, &value.DurationMinutes,
		&value.Timezone, &value.CustomerName, &value.CustomerEmail, &value.CustomerPhone,
		&value.MeetRequested, &value.Notes, &value.CancellationReason,
		&value.CreatedBy, &value.CreatedAt, &value.UpdatedAt,
	)
	if err != nil {
		return domain.Booking{}, repositoryhelpers.MapError(err)
	}
	allocationTable := "app.scheduling_booking_resource_allocations"
	aggregateColumn := "booking_id"
	aggregateID := value.ID
	if value.SessionID != nil {
		allocationTable = "app.scheduling_session_resource_allocations"
		aggregateColumn = "session_id"
		aggregateID = *value.SessionID
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT resource_id,allocation_mode,units
		FROM %s
		WHERE org_id=$1 AND %s=$2
		ORDER BY resource_id`, allocationTable, aggregateColumn),
		organizationID, aggregateID,
	)
	if err != nil {
		return domain.Booking{}, repositoryhelpers.MapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var allocation domain.Allocation
		if err := rows.Scan(&allocation.ResourceID, &allocation.Mode, &allocation.Units); err != nil {
			return domain.Booking{}, repositoryhelpers.MapError(err)
		}
		value.Allocations = append(value.Allocations, allocation)
	}
	return value, repositoryhelpers.MapError(rows.Err())
}

func (r *PostgresRepository) ListBookings(
	ctx context.Context,
	organizationID string,
	branchID uuid.UUID,
	from, until time.Time,
) ([]domain.Booking, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT id FROM app.scheduling_bookings
		WHERE org_id=$1 AND branch_id=$2
		  AND tstzrange(starts_at,ends_at,'[)') && tstzrange($3,$4,'[)')
		ORDER BY starts_at,id`,
		organizationID, branchID, from, until,
	)
	if err != nil {
		return nil, repositoryhelpers.MapError(err)
	}
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, repositoryhelpers.MapError(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, repositoryhelpers.MapError(err)
	}
	rows.Close()
	result := make([]domain.Booking, 0, len(ids))
	for _, id := range ids {
		value, err := getBookingTx(ctx, tx, organizationID, id)
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

func (r *PostgresRepository) GetGroupSession(
	ctx context.Context,
	organizationID string,
	id uuid.UUID,
) (domain.GroupSession, []domain.Allocation, error) {
	tx, err := repositoryhelpers.BeginTenant(ctx, r.pool, organizationID)
	if err != nil {
		return domain.GroupSession{}, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	session, err := getGroupSessionTx(ctx, tx, organizationID, id, false)
	if err != nil {
		return domain.GroupSession{}, nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT resource_id,allocation_mode,units
		FROM app.scheduling_session_resource_allocations
		WHERE org_id=$1 AND session_id=$2
		ORDER BY resource_id`,
		organizationID, id,
	)
	if err != nil {
		return domain.GroupSession{}, nil, repositoryhelpers.MapError(err)
	}
	defer rows.Close()
	allocations := make([]domain.Allocation, 0)
	for rows.Next() {
		var value domain.Allocation
		if err := rows.Scan(&value.ResourceID, &value.Mode, &value.Units); err != nil {
			return domain.GroupSession{}, nil, repositoryhelpers.MapError(err)
		}
		allocations = append(allocations, value)
	}
	if err := rows.Err(); err != nil {
		return domain.GroupSession{}, nil, repositoryhelpers.MapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.GroupSession{}, nil, err
	}
	return session, allocations, nil
}

func getGroupSessionTx(
	ctx context.Context,
	tx pgx.Tx,
	organizationID string,
	id uuid.UUID,
	forUpdate bool,
) (domain.GroupSession, error) {
	query := `
		SELECT org_id,id,branch_id,service_id,starts_at,ends_at,capacity,booked,version,status
		FROM app.scheduling_group_sessions
		WHERE org_id=$1 AND id=$2`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var value domain.GroupSession
	err := tx.QueryRow(ctx, query, organizationID, id).Scan(
		&value.OrganizationID, &value.ID, &value.BranchID, &value.ServiceID,
		&value.StartAt, &value.EndAt, &value.Capacity, &value.Booked,
		&value.Version, &value.Status,
	)
	return value, repositoryhelpers.MapError(err)
}

func dateOnly(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format("2006-01-02")
}
