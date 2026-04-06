package accounts

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/devpablocristo/core/errors/go/domainerr"
	accountsdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/accounts/usecases/domain"
)

type RepositoryPort interface {
	List(ctx context.Context, orgID uuid.UUID, accountType, entityType string, onlyNonZero bool, limit int) ([]accountsdomain.Account, error)
	ListMovements(ctx context.Context, orgID, accountID uuid.UUID, limit int) ([]accountsdomain.Movement, error)
	CreateOrAdjust(ctx context.Context, in accountsdomain.Account, amount float64, description, actor string) (accountsdomain.Account, error)
}

type Usecases struct{ repo RepositoryPort }

func NewUsecases(repo RepositoryPort) *Usecases { return &Usecases{repo: repo} }

func (u *Usecases) List(ctx context.Context, orgID uuid.UUID, accountType, entityType string, onlyNonZero bool, limit int) ([]accountsdomain.Account, error) {
	accountType = normalizeAccountType(accountType)
	entityType = normalizeEntityType(entityType)
	if accountType != "" && !isSupportedAccountType(accountType) {
		return nil, domainerr.Validation("invalid type")
	}
	if entityType != "" && !isSupportedEntityType(entityType) {
		return nil, domainerr.Validation("invalid entity_type")
	}
	if accountType != "" && entityType != "" && !isCompatibleAccountTypeAndEntityType(accountType, entityType) {
		return nil, domainerr.Validation("type and entity_type are inconsistent")
	}
	return u.repo.List(ctx, orgID, accountType, entityType, onlyNonZero, limit)
}

func (u *Usecases) Debtors(ctx context.Context, orgID uuid.UUID, limit int) ([]accountsdomain.Account, error) {
	return u.repo.List(ctx, orgID, "receivable", "customer", true, limit)
}

func (u *Usecases) Movements(ctx context.Context, orgID, accountID uuid.UUID, limit int) ([]accountsdomain.Movement, error) {
	return u.repo.ListMovements(ctx, orgID, accountID, limit)
}

func (u *Usecases) CreateOrAdjust(ctx context.Context, in accountsdomain.Account, amount float64, description, actor string) (accountsdomain.Account, error) {
	if in.OrgID == uuid.Nil {
		return accountsdomain.Account{}, domainerr.Validation("org_id is required")
	}
	in.Type = normalizeAccountType(in.Type)
	if !isSupportedAccountType(in.Type) {
		return accountsdomain.Account{}, domainerr.Validation("invalid type")
	}
	in.EntityType = normalizeEntityType(in.EntityType)
	if in.EntityType == "" {
		in.EntityType = entityTypeFromAccountType(in.Type)
	}
	if !isSupportedEntityType(in.EntityType) || !isCompatibleAccountTypeAndEntityType(in.Type, in.EntityType) {
		return accountsdomain.Account{}, domainerr.Validation("invalid entity_type")
	}
	if in.EntityID == uuid.Nil {
		return accountsdomain.Account{}, domainerr.Validation("entity_id is required")
	}
	if strings.TrimSpace(in.EntityName) == "" {
		return accountsdomain.Account{}, domainerr.Validation("entity_name is required")
	}
	if amount <= 0 {
		return accountsdomain.Account{}, domainerr.Validation("amount must be > 0")
	}
	return u.repo.CreateOrAdjust(ctx, in, amount, description, actor)
}

func normalizeAccountType(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func normalizeEntityType(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func isSupportedAccountType(value string) bool {
	return value == "receivable" || value == "payable"
}

func isSupportedEntityType(value string) bool {
	return value == "customer" || value == "supplier"
}

func entityTypeFromAccountType(accountType string) string {
	switch normalizeAccountType(accountType) {
	case "receivable":
		return "customer"
	case "payable":
		return "supplier"
	default:
		return ""
	}
}

func accountTypeFromEntityType(entityType string) string {
	switch normalizeEntityType(entityType) {
	case "customer":
		return "receivable"
	case "supplier":
		return "payable"
	default:
		return ""
	}
}

func isCompatibleAccountTypeAndEntityType(accountType, entityType string) bool {
	return entityTypeFromAccountType(accountType) == normalizeEntityType(entityType)
}
