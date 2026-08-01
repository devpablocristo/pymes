package helpers

import (
	"context"
	"errors"
	"fmt"

	repositorymodels "github.com/devpablocristo/pymes/v3/backend/internal/organization/repository/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func BeginTenant(
	ctx context.Context,
	pool *pgxpool.Pool,
	organizationID string,
) (pgx.Tx, error) {
	if ctx == nil || pool == nil || organizationID == "" {
		return nil, fmt.Errorf("organization feature tenant is required")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(
		ctx,
		"SELECT set_config('app.org_id',$1,true)",
		organizationID,
	); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}

func ScanFeatureFlags(row pgx.Row) (domain.FeatureFlags, error) {
	var model repositorymodels.FeatureFlags
	err := row.Scan(
		&model.OrganizationID,
		&model.SchedulingEnabled,
		&model.WhatsAppEnabled,
		&model.GoogleCalendarEnabled,
		&model.FiscalRealEnabled,
		&model.Version,
		&model.UpdatedAt,
		&model.UpdatedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.FeatureFlags{}, domain.ErrUnknown
	}
	if err != nil {
		return domain.FeatureFlags{}, err
	}
	return domain.FeatureFlags{
		OrganizationID:        model.OrganizationID,
		SchedulingEnabled:     model.SchedulingEnabled,
		WhatsAppEnabled:       model.WhatsAppEnabled,
		GoogleCalendarEnabled: model.GoogleCalendarEnabled,
		FiscalRealEnabled:     model.FiscalRealEnabled,
		Version:               model.Version,
		UpdatedAt:             model.UpdatedAt,
		UpdatedBy:             model.UpdatedBy,
	}, nil
}
