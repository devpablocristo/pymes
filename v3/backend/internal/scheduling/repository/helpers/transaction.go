package helpers

import (
	"context"
	"errors"
	"fmt"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func BeginTenant(ctx context.Context, pool *pgxpool.Pool, organizationID string) (pgx.Tx, error) {
	if pool == nil || organizationID == "" {
		return nil, domain.NewError(domain.CodeValidation, "database and organization are required")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin scheduling transaction: %w", err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("set scheduling tenant context: %w", err)
	}
	return tx, nil
}

func MapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.WrapError(domain.CodeNotFound, "scheduling record was not found", err)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23P01":
		return domain.WrapError(domain.CodeResourceConflict, "resource is already reserved", err)
	case "23503":
		return domain.WrapError(domain.CodeValidation, "referenced scheduling record does not exist", err)
	case "23505":
		return domain.WrapError(domain.CodeSlotConflict, "scheduling record conflicts with existing data", err)
	case "23514":
		return domain.WrapError(domain.CodeValidation, "scheduling invariant failed", err)
	default:
		return err
	}
}
