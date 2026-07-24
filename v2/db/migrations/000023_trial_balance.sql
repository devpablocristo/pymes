-- Support the tenant-scoped Balance de sumas y saldos aggregation. Posted
-- journal entries remain the only source of truth; this covering index lets
-- the date-scoped entry scan reach account amounts without a second balance
-- table or an in-memory tenant-wide load.

CREATE INDEX accounting_journal_lines_trial_balance_idx
    ON accounting.journal_lines (
        org_id,
        journal_entry_id,
        account_id
    ) INCLUDE (
        debit_amount,
        credit_amount
    );
