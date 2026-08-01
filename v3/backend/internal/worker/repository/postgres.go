// Package repository contains PostgreSQL adapters for worker operational ports.
package repository

import (
	"context"

	workerdomain "github.com/devpablocristo/pymes/v3/backend/internal/worker/domain"
	workerports "github.com/devpablocristo/pymes/v3/backend/internal/worker/ports"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	_ workerports.Readiness     = (*Operations)(nil)
	_ workerports.MetricsReader = (*Operations)(nil)
)

type Operations struct {
	Pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Operations {
	return &Operations{Pool: pool}
}

func (o *Operations) Ready(ctx context.Context) error {
	return o.Pool.Ping(ctx)
}

func (o *Operations) Collect(
	ctx context.Context,
) (workerdomain.Metrics, error) {
	// Suspended organizations are excluded from leasing, but their backlog
	// remains observable so an operational alert cannot be hidden.
	rows, err := o.Pool.Query(
		ctx, `SELECT id FROM app.organizations ORDER BY id`,
	)
	if err != nil {
		return workerdomain.Metrics{}, err
	}
	var organizations []string
	for rows.Next() {
		var organizationID string
		if err := rows.Scan(&organizationID); err != nil {
			rows.Close()
			return workerdomain.Metrics{}, err
		}
		organizations = append(organizations, organizationID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return workerdomain.Metrics{}, err
	}
	rows.Close()

	var result workerdomain.Metrics
	for _, organizationID := range organizations {
		tx, err := o.Pool.Begin(ctx)
		if err != nil {
			return workerdomain.Metrics{}, err
		}
		if _, err = tx.Exec(
			ctx,
			"SELECT set_config('app.org_id',$1,true)",
			organizationID,
		); err != nil {
			_ = tx.Rollback(ctx)
			return workerdomain.Metrics{}, err
		}
		var current workerdomain.Metrics
		err = tx.QueryRow(ctx, `
			SELECT
			  (SELECT count(*) FROM app.outbox WHERE published_at IS NULL),
			  (SELECT count(*) FROM app.outbox
			   WHERE published_at IS NULL AND lease_expires_at > now()),
			  (SELECT count(*) FROM app.outbox
			   WHERE published_at IS NULL AND attempts > 1),
			  (SELECT count(*) FROM app.outbox_dead_letters),
			  COALESCE((
			    SELECT EXTRACT(EPOCH FROM now() - min(created_at))
			    FROM app.outbox WHERE published_at IS NULL
			  ), 0),
			  (SELECT count(*) FROM app.sales
			   WHERE status = 'fiscal_uncertain'),
			  (SELECT count(*) FROM app.accounting_application_commands
			   WHERE status = 'pending'),
			  (SELECT count(*) FROM app.accounting_reversals
			   WHERE status = 'requested')`).
			Scan(
				&current.OutboxPending,
				&current.OutboxLeased,
				&current.OutboxRetrying,
				&current.OutboxDeadLetters,
				&current.OutboxOldestAgeSeconds,
				&current.FiscalUncertain,
				&current.ApplicationPending,
				&current.ReversalPending,
			)
		if err != nil {
			_ = tx.Rollback(ctx)
			return workerdomain.Metrics{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return workerdomain.Metrics{}, err
		}
		result.OutboxPending += current.OutboxPending
		result.OutboxLeased += current.OutboxLeased
		result.OutboxRetrying += current.OutboxRetrying
		result.OutboxDeadLetters += current.OutboxDeadLetters
		if current.OutboxOldestAgeSeconds > result.OutboxOldestAgeSeconds {
			result.OutboxOldestAgeSeconds = current.OutboxOldestAgeSeconds
		}
		result.FiscalUncertain += current.FiscalUncertain
		result.ApplicationPending += current.ApplicationPending
		result.ReversalPending += current.ReversalPending
	}
	return result, nil
}
