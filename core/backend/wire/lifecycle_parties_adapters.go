// Ola C step 9 — lifecycle.RepositoryPort adapters for customers + suppliers.
//
// Both modules share the `parties` table (with a type discriminator) and
// their SoftDelete / Restore / HardDelete run multi-table transactions
// (party_roles, party_persons, party_organizations, accounts). The generic
// lifecycle.SoftDeleter assumes a single UPDATE on one table and cannot
// preserve that transactional logic, so we wrap the existing
// customers.Repository and suppliers.Repository here.
//
// The adapters discard the `at` time.Time argument to SoftDelete — pymes'
// repository uses now() in SQL and returns a single row affected. This is
// a one-way information loss (lifecycle.Service emits OccurredAt in the
// audit entry instead), and it preserves the existing semantics.
package wire

import (
	"context"
	"errors"
	"time"

	lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"
	"github.com/devpablocristo/pymes/core/backend/internal/customers"
	"github.com/devpablocristo/pymes/core/backend/internal/suppliers"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// customersLifecycleRepo wraps *customers.Repository to satisfy lifecycle.RepositoryPort.
// db is held alongside the repository to power IsArchived without requiring a
// new exported method on customers.Repository.
type customersLifecycleRepo struct {
	repo *customers.Repository
	db   *gorm.DB
}

func (a *customersLifecycleRepo) SoftDelete(ctx context.Context, tenantID, resourceID uuid.UUID, _ time.Time) error {
	return a.repo.SoftDelete(ctx, tenantID, resourceID)
}

func (a *customersLifecycleRepo) Restore(ctx context.Context, tenantID, resourceID uuid.UUID) error {
	return a.repo.Restore(ctx, tenantID, resourceID)
}

func (a *customersLifecycleRepo) HardDelete(ctx context.Context, tenantID, resourceID uuid.UUID) error {
	return a.repo.HardDelete(ctx, tenantID, resourceID)
}

func (a *customersLifecycleRepo) IsArchived(ctx context.Context, tenantID, resourceID uuid.UUID) (bool, error) {
	return isPartyArchived(ctx, a.db, tenantID, resourceID)
}

// suppliersLifecycleRepo wraps *suppliers.Repository to satisfy lifecycle.RepositoryPort.
type suppliersLifecycleRepo struct {
	repo *suppliers.Repository
	db   *gorm.DB
}

func (a *suppliersLifecycleRepo) SoftDelete(ctx context.Context, tenantID, resourceID uuid.UUID, _ time.Time) error {
	return a.repo.SoftDelete(ctx, tenantID, resourceID)
}

func (a *suppliersLifecycleRepo) Restore(ctx context.Context, tenantID, resourceID uuid.UUID) error {
	return a.repo.Restore(ctx, tenantID, resourceID)
}

func (a *suppliersLifecycleRepo) HardDelete(ctx context.Context, tenantID, resourceID uuid.UUID) error {
	return a.repo.HardDelete(ctx, tenantID, resourceID)
}

func (a *suppliersLifecycleRepo) IsArchived(ctx context.Context, tenantID, resourceID uuid.UUID) (bool, error) {
	return isPartyArchived(ctx, a.db, tenantID, resourceID)
}

// isPartyArchived is the shared probe over the parties table.
func isPartyArchived(ctx context.Context, db *gorm.DB, tenantID, resourceID uuid.UUID) (bool, error) {
	var archived bool
	err := db.WithContext(ctx).
		Raw(`SELECT deleted_at IS NOT NULL FROM parties WHERE org_id = ? AND id = ?`, tenantID, resourceID).
		Scan(&archived).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, gorm.ErrRecordNotFound
		}
		return false, err
	}
	return archived, nil
}

// Compile-time guarantees.
var (
	_ lifecycle.RepositoryPort = (*customersLifecycleRepo)(nil)
	_ lifecycle.RepositoryPort = (*suppliersLifecycleRepo)(nil)
)
