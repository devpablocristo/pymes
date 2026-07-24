-- Support the account-specific, ascending General Ledger query. Journal
-- entries remain the source of truth; these indexes only make the tenant- and
-- account-scoped read path efficient.

CREATE INDEX accounting_journal_lines_general_ledger_idx
    ON accounting.journal_lines (
        org_id,
        account_id,
        journal_entry_id,
        line_no,
        id
    ) INCLUDE (
        debit_amount,
        credit_amount,
        description
    );

CREATE INDEX accounting_journal_entries_general_ledger_idx
    ON accounting.journal_entries (
        org_id,
        entry_date,
        entry_number,
        id
    ) INCLUDE (
        entry_kind,
        source_type,
        reference,
        description
    );
