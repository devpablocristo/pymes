package accounts

import (
	"context"
	"strings"

	"github.com/google/uuid"

	accountsdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/accounts/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/pkg/apperror"
)

type RepositoryPort interface {
	List(ctx context.Context, orgID uuid.UUID, accountType, entityType string, onlyNonZero bool, limit int) ([]accountsdomain.Account, error)
	ListMovements(ctx context.Context, orgID, accountID uuid.UUID, limit int) ([]accountsdomain.Movement, error)
	CreateOrAdjust(ctx context.Context, in accountsdomain.Account, amount float64, description, actor string) (accountsdomain.Account, error)
}

type Usecases struct{ repo RepositoryPort }

func NewUsecases(repo RepositoryPort) *Usecases { return &Usecases{repo: repo} }

func (u *Usecases) List(ctx context.Context, orgID uuid.UUID, accountType, entityType string, onlyNonZero bool, limit int) ([]accountsdomain.Account, error) {
	return u.repo.List(ctx, orgID, strings.TrimSpace(accountType), strings.TrimSpace(entityType), onlyNonZero, limit)
}

func (u *Usecases) Debtors(ctx context.Context, orgID uuid.UUID, limit int) ([]accountsdomain.Account, error) {
	return u.repo.List(ctx, orgID, "receivable", "customer", true, limit)
}

func (u *Usecases) Movements(ctx context.Context, orgID, accountID uuid.UUID, limit int) ([]accountsdomain.Movement, error) {
	return u.repo.ListMovements(ctx, orgID, accountID, limit)
}

func (u *Usecases) CreateOrAdjust(ctx context.Context, in accountsdomain.Account, amount float64, description, actor string) (accountsdomain.Account, error) {
	if in.OrgID == uuid.Nil {
		return accountsdomain.Account{}, apperror.NewBadInput("org_id is required")
	}
	if strings.TrimSpace(in.Type) != "receivable" && strings.TrimSpace(in.Type) != "payable" {
		return accountsdomain.Account{}, apperror.NewBadInput("invalid type")
	}
	if strings.TrimSpace(in.EntityType) != "customer" && strings.TrimSpace(in.EntityType) != "supplier" {
		return accountsdomain.Account{}, apperror.NewBadInput("invalid entity_type")
	}
	if in.EntityID == uuid.Nil {
		return accountsdomain.Account{}, apperror.NewBadInput("entity_id is required")
	}
	if strings.TrimSpace(in.EntityName) == "" {
		return accountsdomain.Account{}, apperror.NewBadInput("entity_name is required")
	}
	if amount <= 0 {
		return accountsdomain.Account{}, apperror.NewBadInput("amount must be > 0")
	}
	return u.repo.CreateOrAdjust(ctx, in, amount, description, actor)
}
