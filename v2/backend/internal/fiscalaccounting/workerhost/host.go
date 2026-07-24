package workerhost

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscalaccounting"
)

const maxOrganizationBatchSize = 1000

type organizationLister func(context.Context, int) ([]uuid.UUID, error)

type tenantRunner func(
	context.Context,
	uuid.UUID,
) (fiscalaccounting.Result, error)

type Host struct {
	listOrganizations organizationLister
	runTenant         tenantRunner
	organizationBatch int
}

type Result struct {
	OrganizationsFound int
	Processed          int
	NoWork             int
	Failed             int
}

func New(
	pool *pgxpool.Pool,
	worker *fiscalaccounting.Worker,
	organizationBatch int,
) (*Host, error) {
	if pool == nil {
		return nil, errors.New("fiscal accounting host PostgreSQL pool is required")
	}
	if worker == nil {
		return nil, errors.New("fiscal accounting host worker is required")
	}
	return newHost(
		func(ctx context.Context, limit int) ([]uuid.UUID, error) {
			return pendingOrganizations(ctx, pool, limit)
		},
		worker.RunOnce,
		organizationBatch,
	)
}

func newHost(
	listOrganizations organizationLister,
	runTenant tenantRunner,
	organizationBatch int,
) (*Host, error) {
	if listOrganizations == nil {
		return nil, errors.New("fiscal accounting organization lister is required")
	}
	if runTenant == nil {
		return nil, errors.New("fiscal accounting tenant runner is required")
	}
	if organizationBatch < 1 || organizationBatch > maxOrganizationBatchSize {
		return nil, fmt.Errorf(
			"fiscal accounting organization batch must be between 1 and %d",
			maxOrganizationBatchSize,
		)
	}
	return &Host{
		listOrganizations: listOrganizations,
		runTenant:         runTenant,
		organizationBatch: organizationBatch,
	}, nil
}

func pendingOrganizations(
	ctx context.Context,
	pool *pgxpool.Pool,
	limit int,
) ([]uuid.UUID, error) {
	rows, err := pool.Query(ctx, `
		SELECT org_id
		  FROM fiscal.pending_organizations($1)`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list organizations with fiscal accounting work: %w",
			err,
		)
	}
	defer rows.Close()

	organizations := make([]uuid.UUID, 0, limit)
	for rows.Next() {
		var organizationID uuid.UUID
		if err := rows.Scan(&organizationID); err != nil {
			return nil, fmt.Errorf(
				"scan fiscal accounting organization: %w",
				err,
			)
		}
		if organizationID == uuid.Nil {
			return nil, errors.New(
				"fiscal accounting organization discovery returned an empty id",
			)
		}
		organizations = append(organizations, organizationID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate fiscal accounting organizations: %w",
			err,
		)
	}
	return organizations, nil
}

func (host *Host) RunOnce(ctx context.Context) (Result, error) {
	organizations, err := host.listOrganizations(
		ctx,
		host.organizationBatch,
	)
	if err != nil {
		return Result{}, err
	}

	result := Result{OrganizationsFound: len(organizations)}
	var processErrors []error
	for _, organizationID := range organizations {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		_, processErr := host.runTenant(ctx, organizationID)
		switch {
		case processErr == nil:
			result.Processed++
		case errors.Is(processErr, fiscalaccounting.ErrNoWork):
			// pending_organizations also serves the ARCA worker and may return
			// tenants whose fiscal-accounting work is not retryable yet.
			result.NoWork++
		default:
			result.Failed++
			processErrors = append(
				processErrors,
				fmt.Errorf(
					"process fiscal accounting tenant %s: %w",
					organizationID,
					processErr,
				),
			)
		}
	}
	return result, errors.Join(processErrors...)
}

func (host *Host) Run(
	ctx context.Context,
	pollInterval time.Duration,
	observe func(Result, error),
) error {
	if pollInterval <= 0 {
		return errors.New(
			"fiscal accounting host poll interval must be positive",
		)
	}
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			result, err := host.RunOnce(ctx)
			if ctx.Err() != nil {
				return nil
			}
			if observe != nil {
				observe(result, err)
			}
			timer.Reset(pollInterval)
		}
	}
}
