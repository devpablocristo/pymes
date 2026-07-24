package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
)

type scanner interface {
	Scan(...any) error
}

const accountColumns = `
	id,
	code,
	name,
	account_class,
	normal_balance,
	monetary_class,
	parent_id,
	posting_allowed,
	version,
	archived_at,
	created_at,
	updated_at
`

func (repository *Repository) ListAccounts(
	ctx context.Context,
	includeArchived bool,
) ([]accounting.Account, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT `+accountColumns+`
		  FROM accounting.accounts
		 WHERE org_id = $1
		   AND trashed_at IS NULL
		   AND ($2 OR archived_at IS NULL)
		 ORDER BY code, id
	`, repository.orgID, includeArchived)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	accounts := make([]accounting.Account, 0)
	for rows.Next() {
		account, scanErr := scanAccount(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return accounts, nil
}

func (repository *Repository) GetAccount(
	ctx context.Context,
	id uuid.UUID,
) (accounting.Account, error) {
	account, err := scanAccount(repository.tx.QueryRow(ctx, `
		SELECT `+accountColumns+`
		  FROM accounting.accounts
		 WHERE org_id = $1
		   AND id = $2
		   AND trashed_at IS NULL
	`, repository.orgID, id))
	if err != nil {
		return accounting.Account{}, mapError(err)
	}
	return account, nil
}

func (repository *Repository) CreateAccount(
	ctx context.Context,
	account accounting.Account,
) (accounting.Account, error) {
	created, err := scanAccount(repository.tx.QueryRow(ctx, `
		INSERT INTO accounting.accounts (
			org_id,
			id,
			code,
			name,
			account_class,
			parent_id,
			normal_balance,
			monetary_class,
			posting_allowed
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+accountColumns,
		repository.orgID,
		account.ID,
		account.Code,
		account.Name,
		account.Class,
		account.ParentID,
		account.NormalBalance,
		account.Monetary,
		account.Postable,
	))
	if err != nil {
		return accounting.Account{}, mapError(err)
	}
	return created, nil
}

func (repository *Repository) UpdateAccount(
	ctx context.Context,
	account accounting.Account,
	expectedVersion int64,
) (accounting.Account, error) {
	updated, err := scanAccount(repository.tx.QueryRow(ctx, `
		UPDATE accounting.accounts
		   SET code = $3,
		       name = $4,
		       account_class = $5,
		       parent_id = $6,
		       normal_balance = $7,
		       monetary_class = $8,
		       posting_allowed = $9,
		       version = version + 1,
		       updated_at = now()
		 WHERE org_id = $1
		   AND id = $2
		   AND version = $10
		   AND trashed_at IS NULL
		RETURNING `+accountColumns,
		repository.orgID,
		account.ID,
		account.Code,
		account.Name,
		account.Class,
		account.ParentID,
		account.NormalBalance,
		account.Monetary,
		account.Postable,
		expectedVersion,
	))
	if err != nil {
		return accounting.Account{}, optimisticError(err)
	}
	return updated, nil
}

func (repository *Repository) AccountUsage(
	ctx context.Context,
	id uuid.UUID,
) (postings int64, mappings int64, children int64, err error) {
	err = repository.tx.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM accounting.journal_lines
			  WHERE org_id = $1 AND account_id = $2),
			(SELECT count(*) FROM accounting.account_mappings
			  WHERE org_id = $1 AND account_id = $2),
			(SELECT count(*) FROM accounting.accounts
			  WHERE org_id = $1 AND parent_id = $2 AND trashed_at IS NULL)
	`, repository.orgID, id).Scan(&postings, &mappings, &children)
	if err != nil {
		return 0, 0, 0, mapError(err)
	}
	return postings, mappings, children, nil
}

func (repository *Repository) ArchiveAccount(
	ctx context.Context,
	id uuid.UUID,
	expectedVersion int64,
	at time.Time,
	_ string,
) (accounting.Account, error) {
	account, err := scanAccount(repository.tx.QueryRow(ctx, `
		UPDATE accounting.accounts
		   SET archived_at = $4,
		       version = version + 1,
		       updated_at = now()
		 WHERE org_id = $1
		   AND id = $2
		   AND version = $3
		   AND archived_at IS NULL
		   AND trashed_at IS NULL
		RETURNING `+accountColumns,
		repository.orgID,
		id,
		expectedVersion,
		at,
	))
	if err != nil {
		return accounting.Account{}, optimisticError(err)
	}
	return account, nil
}

func (repository *Repository) RestoreAccount(
	ctx context.Context,
	id uuid.UUID,
	expectedVersion int64,
	_ string,
) (accounting.Account, error) {
	account, err := scanAccount(repository.tx.QueryRow(ctx, `
		UPDATE accounting.accounts
		   SET archived_at = NULL,
		       version = version + 1,
		       updated_at = now()
		 WHERE org_id = $1
		   AND id = $2
		   AND version = $3
		   AND archived_at IS NOT NULL
		   AND trashed_at IS NULL
		RETURNING `+accountColumns,
		repository.orgID,
		id,
		expectedVersion,
	))
	if err != nil {
		return accounting.Account{}, optimisticError(err)
	}
	return account, nil
}

func (repository *Repository) DeleteUnusedAccount(
	ctx context.Context,
	id uuid.UUID,
	expectedVersion int64,
) error {
	commandTag, err := repository.tx.Exec(ctx, `
		UPDATE accounting.accounts
		   SET trashed_at = now(),
		       archived_at = NULL,
		       version = version + 1,
		       updated_at = now()
		 WHERE org_id = $1
		   AND id = $2
		   AND version = $3
		   AND trashed_at IS NULL
	`, repository.orgID, id, expectedVersion)
	if err != nil {
		return mapError(err)
	}
	if commandTag.RowsAffected() == 0 {
		return accounting.ErrVersionConflict
	}
	return nil
}

func (repository *Repository) ListMappings(
	ctx context.Context,
) ([]accounting.AccountMapping, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT
			mapping.mapping_key,
			mapping.account_id,
			account.code,
			account.name,
			mapping.version,
			mapping.updated_at
		  FROM accounting.account_mappings AS mapping
		  JOIN accounting.accounts AS account
		    ON account.org_id = mapping.org_id
		   AND account.id = mapping.account_id
		 WHERE mapping.org_id = $1
		 ORDER BY mapping.mapping_key
	`, repository.orgID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	result := make([]accounting.AccountMapping, 0)
	for rows.Next() {
		var mapping accounting.AccountMapping
		if err := rows.Scan(
			&mapping.Role,
			&mapping.AccountID,
			&mapping.AccountCode,
			&mapping.AccountName,
			&mapping.Version,
			&mapping.UpdatedAt,
		); err != nil {
			return nil, mapError(err)
		}
		mapping.UpdatedBy = repository.actor
		result = append(result, mapping)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return result, nil
}

func (repository *Repository) GetMappings(
	ctx context.Context,
	roles []string,
) (map[string]accounting.AccountMapping, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT
			mapping.mapping_key,
			mapping.account_id,
			account.code,
			account.name,
			mapping.version,
			mapping.updated_at
		  FROM accounting.account_mappings AS mapping
		  JOIN accounting.accounts AS account
		    ON account.org_id = mapping.org_id
		   AND account.id = mapping.account_id
		 WHERE mapping.org_id = $1
		   AND mapping.mapping_key = ANY($2::text[])
		   AND account.archived_at IS NULL
		   AND account.trashed_at IS NULL
	`, repository.orgID, roles)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	result := make(map[string]accounting.AccountMapping, len(roles))
	for rows.Next() {
		var mapping accounting.AccountMapping
		if err := rows.Scan(
			&mapping.Role,
			&mapping.AccountID,
			&mapping.AccountCode,
			&mapping.AccountName,
			&mapping.Version,
			&mapping.UpdatedAt,
		); err != nil {
			return nil, mapError(err)
		}
		mapping.UpdatedBy = repository.actor
		result[mapping.Role] = mapping
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	for _, role := range roles {
		if _, ok := result[role]; !ok {
			return nil, fmt.Errorf("%w: %s", accounting.ErrMappingMissing, role)
		}
	}
	return result, nil
}

func (repository *Repository) SetMapping(
	ctx context.Context,
	mapping accounting.AccountMapping,
	expectedVersion int64,
) (accounting.AccountMapping, error) {
	var row scanner
	if expectedVersion <= 0 {
		row = repository.tx.QueryRow(ctx, `
			INSERT INTO accounting.account_mappings (
				org_id,
				mapping_key,
				account_id
			)
			VALUES ($1, $2, $3)
			RETURNING mapping_key, account_id, version, updated_at
		`, repository.orgID, mapping.Role, mapping.AccountID)
	} else {
		row = repository.tx.QueryRow(ctx, `
			UPDATE accounting.account_mappings
			   SET account_id = $3,
			       version = version + 1,
			       updated_at = now()
			 WHERE org_id = $1
			   AND mapping_key = $2
			   AND version = $4
			RETURNING mapping_key, account_id, version, updated_at
		`, repository.orgID, mapping.Role, mapping.AccountID, expectedVersion)
	}
	if err := row.Scan(&mapping.Role, &mapping.AccountID, &mapping.Version, &mapping.UpdatedAt); err != nil {
		return accounting.AccountMapping{}, optimisticError(err)
	}
	account, err := repository.GetAccount(ctx, mapping.AccountID)
	if err != nil {
		return accounting.AccountMapping{}, err
	}
	mapping.AccountCode = account.Code
	mapping.AccountName = account.Name
	mapping.UpdatedBy = repository.actor
	return mapping, nil
}

func scanAccount(row scanner) (accounting.Account, error) {
	var account accounting.Account
	if err := row.Scan(
		&account.ID,
		&account.Code,
		&account.Name,
		&account.Class,
		&account.NormalBalance,
		&account.Monetary,
		&account.ParentID,
		&account.Postable,
		&account.Version,
		&account.ArchivedAt,
		&account.CreatedAt,
		&account.UpdatedAt,
	); err != nil {
		return accounting.Account{}, mapError(err)
	}
	return account, nil
}

func optimisticError(err error) error {
	mapped := mapError(err)
	if mapped == accounting.ErrNotFound {
		return accounting.ErrVersionConflict
	}
	return mapped
}
