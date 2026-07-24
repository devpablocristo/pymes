package administration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	platformlifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	lifecycleScope       = "pymes"
	TenantResourceType   = "pymes.tenant"
	UserResourceType     = "pymes.user"
	defaultRetentionDays = 0
)

type lifecycleActorKey struct{}

func contextWithLifecycleActor(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, lifecycleActorKey{}, strings.TrimSpace(subject))
}

type lifecycleRepository struct {
	pool         *pgxpool.Pool
	resourceType string
	provider     Provider
}

func (repository *lifecycleRepository) Archive(
	ctx context.Context,
	scope string,
	resourceID uuid.UUID,
	at time.Time,
) error {
	return repository.mutate(ctx, scope, resourceID, func(ctx context.Context, tx pgx.Tx) error {
		if err := repository.protectLastOwner(ctx, tx, resourceID); err != nil {
			return err
		}
		return repository.execOne(ctx, tx, resourceID, `
			SET archived_at = $2, updated_at = now()
			WHERE id = $1 AND archived_at IS NULL AND trashed_at IS NULL
		`, at.UTC())
	})
}

func (repository *lifecycleRepository) Unarchive(
	ctx context.Context,
	scope string,
	resourceID uuid.UUID,
) error {
	return repository.mutate(ctx, scope, resourceID, func(ctx context.Context, tx pgx.Tx) error {
		return repository.execOne(ctx, tx, resourceID, `
			SET archived_at = NULL, updated_at = now()
			WHERE id = $1 AND archived_at IS NOT NULL AND trashed_at IS NULL
		`)
	})
}

func (repository *lifecycleRepository) Trash(
	ctx context.Context,
	scope string,
	resourceID uuid.UUID,
	at time.Time,
	purgeAfter *time.Time,
) error {
	return repository.mutate(ctx, scope, resourceID, func(ctx context.Context, tx pgx.Tx) error {
		if err := repository.protectLastOwner(ctx, tx, resourceID); err != nil {
			return err
		}
		return repository.execOne(ctx, tx, resourceID, `
			SET archived_at = NULL, trashed_at = $2, purge_after = $3, updated_at = now()
			WHERE id = $1 AND trashed_at IS NULL
		`, at.UTC(), purgeAfter)
	})
}

func (repository *lifecycleRepository) Restore(
	ctx context.Context,
	scope string,
	resourceID uuid.UUID,
) error {
	return repository.mutate(ctx, scope, resourceID, func(ctx context.Context, tx pgx.Tx) error {
		return repository.execOne(ctx, tx, resourceID, `
			SET trashed_at = NULL, purge_after = NULL, updated_at = now()
			WHERE id = $1 AND trashed_at IS NOT NULL
		`)
	})
}

func (repository *lifecycleRepository) Purge(
	ctx context.Context,
	scope string,
	resourceID uuid.UUID,
) error {
	return repository.mutate(ctx, scope, resourceID, func(ctx context.Context, tx pgx.Tx) error {
		var externalID string
		err := tx.QueryRow(ctx, `
			SELECT coalesce(external_id, '')
			FROM `+repository.table()+`
			WHERE id = $1 AND trashed_at IS NOT NULL
			FOR UPDATE
		`, resourceID).Scan(&externalID)
		if errors.Is(err, pgx.ErrNoRows) {
			return repository.classifyMissing(ctx, tx, resourceID)
		}
		if err != nil {
			return err
		}
		if externalID != "" {
			if repository.provider == nil {
				return ErrProviderBacked
			}
			if repository.resourceType == TenantResourceType {
				err = repository.provider.DeleteOrganization(ctx, externalID)
			} else {
				err = repository.provider.DeleteUser(ctx, externalID)
			}
			if err != nil {
				return fmt.Errorf("purge provider resource: %w", err)
			}
		}
		if repository.resourceType == TenantResourceType {
			if _, err := tx.Exec(ctx, `
				DELETE FROM app.organization_provisioning_requests
				WHERE organization_id = $1
			`, resourceID); err != nil {
				return err
			}
		}
		tag, err := tx.Exec(ctx, `DELETE FROM `+repository.table()+` WHERE id = $1`, resourceID)
		if err != nil {
			return ErrConflict
		}
		if tag.RowsAffected() != 1 {
			return ErrNotFound
		}
		return nil
	})
}

func (repository *lifecycleRepository) State(
	ctx context.Context,
	scope string,
	resourceID uuid.UUID,
) (platformlifecycle.LifecycleState, error) {
	var state platformlifecycle.LifecycleState
	err := repository.withOwnerTx(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		var archived, trashed bool
		err := tx.QueryRow(ctx, `
			SELECT archived_at IS NOT NULL, trashed_at IS NOT NULL
			FROM `+repository.table()+` WHERE id = $1
		`, resourceID).Scan(&archived, &trashed)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		switch {
		case trashed:
			state = platformlifecycle.StateTrashed
		case archived:
			state = platformlifecycle.StateArchived
		default:
			state = platformlifecycle.StateActive
		}
		return nil
	})
	return state, err
}

func (repository *lifecycleRepository) mutate(
	ctx context.Context,
	scope string,
	resourceID uuid.UUID,
	fn func(context.Context, pgx.Tx) error,
) error {
	return repository.withOwnerTx(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, tx)
	})
}

func (repository *lifecycleRepository) withOwnerTx(
	ctx context.Context,
	scope string,
	fn func(context.Context, pgx.Tx) error,
) error {
	if scope != lifecycleScope {
		return ErrForbidden
	}
	subject, _ := ctx.Value(lifecycleActorKey{}).(string)
	if strings.TrimSpace(subject) == "" {
		return ErrForbidden
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.actor_provider', 'clerk', true)`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.actor_subject', $1, true)`, subject); err != nil {
		return err
	}
	var owner bool
	if err := tx.QueryRow(ctx, `SELECT app.is_global_owner('clerk', $1)`, subject).Scan(&owner); err != nil {
		return err
	}
	if !owner {
		return ErrForbidden
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (repository *lifecycleRepository) execOne(
	ctx context.Context,
	tx pgx.Tx,
	resourceID uuid.UUID,
	updateClause string,
	args ...any,
) error {
	queryArgs := append([]any{resourceID}, args...)
	tag, err := tx.Exec(ctx, `UPDATE `+repository.table()+` `+updateClause, queryArgs...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	return repository.classifyMissing(ctx, tx, resourceID)
}

func (repository *lifecycleRepository) classifyMissing(
	ctx context.Context,
	tx pgx.Tx,
	resourceID uuid.UUID,
) error {
	var exists bool
	if err := tx.QueryRow(
		ctx,
		`SELECT EXISTS (SELECT 1 FROM `+repository.table()+` WHERE id = $1)`,
		resourceID,
	).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return ErrConflict
}

func (repository *lifecycleRepository) protectLastOwner(
	ctx context.Context,
	tx pgx.Tx,
	resourceID uuid.UUID,
) error {
	if repository.resourceType != UserResourceType {
		return nil
	}
	var targetIsActiveOwner bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM iam.users AS iam_user
			JOIN app.global_user_roles AS global_role ON global_role.user_id = iam_user.id
			WHERE iam_user.id = $1
			  AND iam_user.status = 'active'
			  AND iam_user.archived_at IS NULL
			  AND iam_user.trashed_at IS NULL
			  AND global_role.status = 'active'
		)
	`, resourceID).Scan(&targetIsActiveOwner); err != nil {
		return err
	}
	if !targetIsActiveOwner {
		return nil
	}
	return ensureAnotherActiveOwner(ctx, tx)
}

func (repository *lifecycleRepository) table() string {
	if repository.resourceType == TenantResourceType {
		return "iam.organizations"
	}
	return "iam.users"
}

type lifecycleAuditAdapter struct {
	pool *pgxpool.Pool
}

func (adapter *lifecycleAuditAdapter) Append(
	ctx context.Context,
	event platformlifecycle.AuditEvent,
) error {
	_, err := adapter.pool.Exec(ctx, `
		INSERT INTO app.lifecycle_audit_events (
			id, scope_id, resource_type, resource_id, action, actor, reason,
			from_state, to_state, retention_expires_at, occurred_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, event.ID, event.TenantID, event.ResourceType, event.ResourceID,
		string(event.Action), event.Actor, event.Reason, string(event.FromState),
		string(event.ToState), event.RetentionExpires, event.OccurredAt)
	return err
}

func newLifecycleService(
	pool *pgxpool.Pool,
	provider Provider,
) (*platformlifecycle.Service, error) {
	repositories := map[string]platformlifecycle.RepositoryPort{
		TenantResourceType: &lifecycleRepository{
			pool: pool, resourceType: TenantResourceType, provider: provider,
		},
		UserResourceType: &lifecycleRepository{
			pool: pool, resourceType: UserResourceType, provider: provider,
		},
	}
	policies := platformlifecycle.NewStaticPolicyRegistry(
		&platformlifecycle.LifecyclePolicy{
			ResourceType:  TenantResourceType,
			AllowArchive:  true,
			AllowTrash:    true,
			AllowPurge:    true,
			RetentionDays: defaultRetentionDays,
		},
		&platformlifecycle.LifecyclePolicy{
			ResourceType:  UserResourceType,
			AllowArchive:  true,
			AllowTrash:    true,
			AllowPurge:    true,
			RetentionDays: defaultRetentionDays,
		},
	)
	return platformlifecycle.NewServiceWithRepos(
		repositories,
		&lifecycleAuditAdapter{pool: pool},
		policies,
	)
}

var _ platformlifecycle.RepositoryPort = (*lifecycleRepository)(nil)
var _ platformlifecycle.AuditPort = (*lifecycleAuditAdapter)(nil)
