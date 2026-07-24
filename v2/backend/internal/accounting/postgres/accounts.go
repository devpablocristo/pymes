package postgres

import (
	"context"
	"fmt"
	"strings"
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
	coalesce(system_key, ''),
	version,
	archived_at,
	trashed_at,
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
		 ORDER BY accounting.account_code_sort_key(code), id
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

func (repository *Repository) ListAccountDetails(
	ctx context.Context,
	includeTrashed bool,
) ([]accounting.AccountDetail, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT `+accountColumns+`,
			(SELECT count(*) FROM accounting.journal_lines
			  WHERE org_id = account.org_id AND account_id = account.id),
			(SELECT count(*) FROM accounting.draft_lines
			  WHERE org_id = account.org_id AND account_id = account.id),
			(SELECT count(*) FROM accounting.account_mappings
			  WHERE org_id = account.org_id AND account_id = account.id),
			(SELECT count(*) FROM accounting.accounts
			  WHERE org_id = account.org_id AND parent_id = account.id),
			(SELECT count(*) FROM accounting.accounts
			  WHERE org_id = account.org_id AND parent_id = account.id
			    AND archived_at IS NULL AND trashed_at IS NULL),
			(SELECT count(*) FROM accounting.financial_accounts
			  WHERE org_id = account.org_id
			    AND ledger_account_id = account.id),
			(SELECT count(*) FROM accounting.financial_accounts
			  WHERE org_id = account.org_id
			    AND ledger_account_id = account.id AND archived_at IS NULL),
			(SELECT count(*) FROM accounting.open_items
			  WHERE org_id = account.org_id AND account_id = account.id),
			(SELECT count(*) FROM accounting.inflation_run_lines
			  WHERE org_id = account.org_id AND account_id = account.id),
			(SELECT count(*) FROM accounting.currency_revaluation_lines
			  WHERE org_id = account.org_id AND account_id = account.id),
			(
				WITH RECURSIVE ancestors AS (
					SELECT parent.id, parent.parent_id,
					       parent.archived_at, parent.trashed_at
					  FROM accounting.accounts AS parent
					 WHERE parent.org_id = account.org_id
					   AND parent.id = account.parent_id
					UNION ALL
					SELECT parent.id, parent.parent_id,
					       parent.archived_at, parent.trashed_at
					  FROM accounting.accounts AS parent
					  JOIN ancestors ON ancestors.parent_id = parent.id
					 WHERE parent.org_id = account.org_id
				)
				SELECT count(*) FROM ancestors
				 WHERE archived_at IS NOT NULL OR trashed_at IS NOT NULL
			),
			ARRAY(
				SELECT mapping.mapping_key
				  FROM accounting.account_mappings AS mapping
				 WHERE mapping.org_id = account.org_id
				   AND mapping.account_id = account.id
				 ORDER BY mapping.mapping_key
			)
		  FROM accounting.accounts AS account
		 WHERE account.org_id = $1
		   AND ($2 OR account.trashed_at IS NULL)
		 ORDER BY accounting.account_code_sort_key(account.code), account.id
	`, repository.orgID, includeTrashed)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	result := make([]accounting.AccountDetail, 0)
	for rows.Next() {
		detail, scanErr := scanAccountDetail(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, detail)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return result, nil
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

func (repository *Repository) GetAccountDetail(
	ctx context.Context,
	id uuid.UUID,
) (accounting.AccountDetail, error) {
	account, err := scanAccount(repository.tx.QueryRow(ctx, `
		SELECT `+accountColumns+`
		  FROM accounting.accounts
		 WHERE org_id = $1
		   AND id = $2
	`, repository.orgID, id))
	if err != nil {
		return accounting.AccountDetail{}, mapError(err)
	}
	usage, err := repository.AccountUsage(ctx, id)
	if err != nil {
		return accounting.AccountDetail{}, err
	}
	rows, err := repository.tx.Query(ctx, `
		SELECT mapping_key
		  FROM accounting.account_mappings
		 WHERE org_id = $1
		   AND account_id = $2
		 ORDER BY mapping_key
	`, repository.orgID, id)
	if err != nil {
		return accounting.AccountDetail{}, mapError(err)
	}
	defer rows.Close()
	roles := make([]string, 0)
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return accounting.AccountDetail{}, mapError(err)
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return accounting.AccountDetail{}, mapError(err)
	}
	return accounting.AccountDetail{
		Account:      account,
		Usage:        usage,
		Capabilities: accounting.BuildAccountCapabilities(account, usage),
		MappingRoles: roles,
	}, nil
}

func (repository *Repository) CreateAccount(
	ctx context.Context,
	account accounting.Account,
) (accounting.Account, error) {
	if err := repository.setAccountingReason(ctx, "Creación de cuenta"); err != nil {
		return accounting.Account{}, err
	}
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
	if err := repository.setAccountingReason(ctx, "Actualización de cuenta"); err != nil {
		return accounting.Account{}, err
	}
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
) (accounting.AccountUsage, error) {
	var usage accounting.AccountUsage
	err := repository.tx.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM accounting.journal_lines
			  WHERE org_id = $1 AND account_id = $2),
			(SELECT count(*) FROM accounting.draft_lines
			  WHERE org_id = $1 AND account_id = $2),
			(SELECT count(*) FROM accounting.account_mappings
			  WHERE org_id = $1 AND account_id = $2),
			(SELECT count(*) FROM accounting.accounts
			  WHERE org_id = $1 AND parent_id = $2),
			(SELECT count(*) FROM accounting.accounts
			  WHERE org_id = $1 AND parent_id = $2
			    AND archived_at IS NULL AND trashed_at IS NULL),
			(SELECT count(*) FROM accounting.financial_accounts
			  WHERE org_id = $1 AND ledger_account_id = $2),
			(SELECT count(*) FROM accounting.financial_accounts
			  WHERE org_id = $1 AND ledger_account_id = $2
			    AND archived_at IS NULL),
			(SELECT count(*) FROM accounting.open_items
			  WHERE org_id = $1 AND account_id = $2),
			(SELECT count(*) FROM accounting.inflation_run_lines
			  WHERE org_id = $1 AND account_id = $2),
			(SELECT count(*) FROM accounting.currency_revaluation_lines
			  WHERE org_id = $1 AND account_id = $2),
			(
				WITH RECURSIVE ancestors AS (
					SELECT parent.id, parent.parent_id,
					       parent.archived_at, parent.trashed_at
					  FROM accounting.accounts AS account
					  JOIN accounting.accounts AS parent
					    ON parent.org_id = account.org_id
					   AND parent.id = account.parent_id
					 WHERE account.org_id = $1 AND account.id = $2
					UNION ALL
					SELECT parent.id, parent.parent_id,
					       parent.archived_at, parent.trashed_at
					  FROM accounting.accounts AS parent
					  JOIN ancestors ON ancestors.parent_id = parent.id
					 WHERE parent.org_id = $1
				)
				SELECT count(*) FROM ancestors
				 WHERE archived_at IS NOT NULL OR trashed_at IS NOT NULL
			)
	`, repository.orgID, id).Scan(
		&usage.JournalLines,
		&usage.DraftLines,
		&usage.Mappings,
		&usage.Children,
		&usage.ActiveChildren,
		&usage.FinancialAccounts,
		&usage.ActiveFinancialAccounts,
		&usage.OpenItems,
		&usage.InflationLines,
		&usage.RevaluationLines,
		&usage.InactiveAncestors,
	)
	if err != nil {
		return accounting.AccountUsage{}, mapError(err)
	}
	return usage, nil
}

func (repository *Repository) ArchiveAccount(
	ctx context.Context,
	id uuid.UUID,
	expectedVersion int64,
	at time.Time,
	reason string,
) (accounting.Account, error) {
	if err := repository.setAccountingReason(ctx, reason); err != nil {
		return accounting.Account{}, err
	}
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
	reason string,
) (accounting.Account, error) {
	if err := repository.setAccountingReason(ctx, reason); err != nil {
		return accounting.Account{}, err
	}
	account, err := scanAccount(repository.tx.QueryRow(ctx, `
		UPDATE accounting.accounts
		   SET archived_at = NULL,
		       trashed_at = NULL,
		       version = version + 1,
		       updated_at = now()
		 WHERE org_id = $1
		   AND id = $2
		   AND version = $3
		   AND (archived_at IS NOT NULL OR trashed_at IS NOT NULL)
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

func (repository *Repository) TrashUnusedAccount(
	ctx context.Context,
	id uuid.UUID,
	expectedVersion int64,
	reason string,
) error {
	if err := repository.setAccountingReason(ctx, reason); err != nil {
		return err
	}
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

func (repository *Repository) ListMappingDefinitions(
	ctx context.Context,
) ([]accounting.AccountMappingDefinition, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT
			role,
			label_es,
			label_en,
			description_es,
			description_en,
			required,
			compatible_account_classes,
			compatible_normal_balances,
			compatible_monetary_classes,
			coalesce(canonical_role, ''),
			is_alias,
			display_order
		  FROM accounting.account_mapping_definitions
		 ORDER BY display_order, role
	`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	result := make([]accounting.AccountMappingDefinition, 0)
	for rows.Next() {
		definition, scanErr := scanMappingDefinition(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, definition)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return result, nil
}

func (repository *Repository) GetMappingDefinition(
	ctx context.Context,
	role string,
) (accounting.AccountMappingDefinition, error) {
	return scanMappingDefinition(repository.tx.QueryRow(ctx, `
		SELECT
			role,
			label_es,
			label_en,
			description_es,
			description_en,
			required,
			compatible_account_classes,
			compatible_normal_balances,
			compatible_monetary_classes,
			coalesce(canonical_role, ''),
			is_alias,
			display_order
		  FROM accounting.account_mapping_definitions
		 WHERE role = $1
	`, role))
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
	if err := repository.setAccountingReason(ctx, mapping.Reason); err != nil {
		return accounting.AccountMapping{}, err
	}
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
		&account.SystemKey,
		&account.Version,
		&account.ArchivedAt,
		&account.TrashedAt,
		&account.CreatedAt,
		&account.UpdatedAt,
	); err != nil {
		return accounting.Account{}, mapError(err)
	}
	account.NodeType = account.EffectiveNodeType()
	account.SystemManaged = account.SystemKey != ""
	return account, nil
}

func scanMappingDefinition(row scanner) (accounting.AccountMappingDefinition, error) {
	var definition accounting.AccountMappingDefinition
	var classes []string
	var balances []string
	var monetaryClasses []string
	if err := row.Scan(
		&definition.Role,
		&definition.LabelES,
		&definition.LabelEN,
		&definition.DescriptionES,
		&definition.DescriptionEN,
		&definition.Required,
		&classes,
		&balances,
		&monetaryClasses,
		&definition.CanonicalRole,
		&definition.Alias,
		&definition.DisplayOrder,
	); err != nil {
		return accounting.AccountMappingDefinition{}, mapError(err)
	}
	for _, class := range classes {
		definition.CompatibleAccountClasses = append(
			definition.CompatibleAccountClasses,
			accounting.AccountClass(class),
		)
	}
	for _, balance := range balances {
		definition.CompatibleNormalBalances = append(
			definition.CompatibleNormalBalances,
			accounting.NormalBalance(balance),
		)
	}
	for _, monetaryClass := range monetaryClasses {
		definition.CompatibleMonetaryClasses = append(
			definition.CompatibleMonetaryClasses,
			accounting.MonetaryClassification(monetaryClass),
		)
	}
	return definition, nil
}

func scanAccountDetail(row scanner) (accounting.AccountDetail, error) {
	var detail accounting.AccountDetail
	if err := row.Scan(
		&detail.Account.ID,
		&detail.Account.Code,
		&detail.Account.Name,
		&detail.Account.Class,
		&detail.Account.NormalBalance,
		&detail.Account.Monetary,
		&detail.Account.ParentID,
		&detail.Account.Postable,
		&detail.Account.SystemKey,
		&detail.Account.Version,
		&detail.Account.ArchivedAt,
		&detail.Account.TrashedAt,
		&detail.Account.CreatedAt,
		&detail.Account.UpdatedAt,
		&detail.Usage.JournalLines,
		&detail.Usage.DraftLines,
		&detail.Usage.Mappings,
		&detail.Usage.Children,
		&detail.Usage.ActiveChildren,
		&detail.Usage.FinancialAccounts,
		&detail.Usage.ActiveFinancialAccounts,
		&detail.Usage.OpenItems,
		&detail.Usage.InflationLines,
		&detail.Usage.RevaluationLines,
		&detail.Usage.InactiveAncestors,
		&detail.MappingRoles,
	); err != nil {
		return accounting.AccountDetail{}, mapError(err)
	}
	detail.Account.NodeType = detail.Account.EffectiveNodeType()
	detail.Account.SystemManaged = detail.Account.SystemKey != ""
	detail.Capabilities = accounting.BuildAccountCapabilities(
		detail.Account,
		detail.Usage,
	)
	return detail, nil
}

func (repository *Repository) setAccountingReason(ctx context.Context, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil
	}
	if _, err := repository.tx.Exec(
		ctx,
		"SELECT set_config('app.accounting_reason', $1, true)",
		reason,
	); err != nil {
		return mapError(err)
	}
	return nil
}

func optimisticError(err error) error {
	mapped := mapError(err)
	if mapped == accounting.ErrNotFound {
		return accounting.ErrVersionConflict
	}
	return mapped
}
