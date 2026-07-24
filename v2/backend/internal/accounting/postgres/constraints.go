package postgres

import "context"

const (
	draftConstraintsImmediate = `
		SET CONSTRAINTS
			accounting.accounting_drafts_currency_consistency,
			accounting.accounting_draft_lines_currency_consistency
		IMMEDIATE
	`
	draftConstraintsDeferred = `
		SET CONSTRAINTS
			accounting.accounting_drafts_currency_consistency,
			accounting.accounting_draft_lines_currency_consistency
		DEFERRED
	`
	journalConstraintsImmediate = `
		SET CONSTRAINTS
			accounting.accounting_journal_entries_valid,
			accounting.accounting_journal_lines_entry_valid,
			accounting.accounting_journal_entries_workflow_invariants,
			accounting.accounting_journal_lines_workflow_invariants,
			accounting.accounting_journal_lines_closed_reconciliation
		IMMEDIATE
	`
	journalConstraintsDeferred = `
		SET CONSTRAINTS
			accounting.accounting_journal_entries_valid,
			accounting.accounting_journal_lines_entry_valid,
			accounting.accounting_journal_entries_workflow_invariants,
			accounting.accounting_journal_lines_workflow_invariants,
			accounting.accounting_journal_lines_closed_reconciliation
		DEFERRED
	`
)

// validateDeferredConstraints forces the selected accounting constraints while
// the repository operation is still on the call stack. This is important for
// caller-owned transactions: otherwise PostgreSQL would report the error only
// from the outer COMMIT, after the accounting service has returned.
//
// Restoring the constraints to deferred mode is equally important. A caller
// may post more than one entry in the same transaction, and an entry row is
// necessarily incomplete until all of its lines have been inserted.
func (repository *Repository) validateDeferredConstraints(
	ctx context.Context,
	immediateCommand string,
	deferredCommand string,
) error {
	if _, err := repository.tx.Exec(ctx, immediateCommand); err != nil {
		return mapError(err)
	}
	if _, err := repository.tx.Exec(ctx, deferredCommand); err != nil {
		return mapError(err)
	}
	return nil
}

func (repository *Repository) validateDraftConstraints(ctx context.Context) error {
	return repository.validateDeferredConstraints(
		ctx,
		draftConstraintsImmediate,
		draftConstraintsDeferred,
	)
}

func (repository *Repository) validateJournalConstraints(ctx context.Context) error {
	return repository.validateDeferredConstraints(
		ctx,
		journalConstraintsImmediate,
		journalConstraintsDeferred,
	)
}
