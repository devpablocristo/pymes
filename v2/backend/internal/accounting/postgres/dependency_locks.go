package postgres

import (
	"bytes"
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
)

func uniqueSortedUUIDs(values []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(values))
	result := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(left, right int) bool {
		return bytes.Compare(result[left][:], result[right][:]) < 0
	})
	return result
}

func journalAccountIDs(lines []accounting.JournalLine) []uuid.UUID {
	values := make([]uuid.UUID, 0, len(lines))
	for _, line := range lines {
		values = append(values, line.AccountID)
	}
	return uniqueSortedUUIDs(values)
}

func (repository *Repository) lockPostingDependencies(
	ctx context.Context,
	lines []accounting.JournalLine,
) error {
	accountIDs := journalAccountIDs(lines)
	if len(accountIDs) == 0 {
		return nil
	}

	rows, err := repository.tx.Query(ctx, `
		SELECT account.id
		  FROM accounting.accounts AS account
		 WHERE account.org_id = $1
		   AND account.id = ANY($2::uuid[])
		 ORDER BY account.id
		 FOR SHARE
	`, repository.orgID, accountIDs)
	if err != nil {
		return mapError(err)
	}
	lockedAccounts := 0
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return mapError(err)
		}
		lockedAccounts++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return mapError(err)
	}
	if lockedAccounts != len(accountIDs) {
		return accounting.ErrAccountNotPostable
	}

	rows, err = repository.tx.Query(ctx, `
		SELECT financial_account.id
		  FROM accounting.financial_accounts AS financial_account
		 WHERE financial_account.org_id = $1
		   AND financial_account.ledger_account_id = ANY($2::uuid[])
		 ORDER BY financial_account.id
		 FOR SHARE
	`, repository.orgID, accountIDs)
	if err != nil {
		return mapError(err)
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return mapError(err)
		}
	}
	rows.Close()
	return mapError(rows.Err())
}

func (repository *Repository) lockReconciliationFinancialAccounts(
	ctx context.Context,
	financialAccountIDs ...uuid.UUID,
) error {
	ids := uniqueSortedUUIDs(financialAccountIDs)
	if len(ids) == 0 {
		return accounting.ErrNotFound
	}
	rows, err := repository.tx.Query(ctx, `
		SELECT financial_account.id
		  FROM accounting.financial_accounts AS financial_account
		 WHERE financial_account.org_id = $1
		   AND financial_account.id = ANY($2::uuid[])
		 ORDER BY financial_account.id
		 FOR UPDATE
	`, repository.orgID, ids)
	if err != nil {
		return mapError(err)
	}
	locked := 0
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return mapError(err)
		}
		locked++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return mapError(err)
	}
	if locked != len(ids) {
		return accounting.ErrNotFound
	}
	return nil
}
