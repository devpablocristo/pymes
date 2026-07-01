-- 0024_ledger_returns_idempotency.down.sql — vuelve al scope de 0023 (sale, payment).

DROP INDEX IF EXISTS uq_journal_entries_idempotency;
CREATE UNIQUE INDEX IF NOT EXISTS uq_journal_entries_idempotency
    ON journal_entries(org_id, source_type, source_id, source_event)
    WHERE source_id IS NOT NULL AND source_type IN ('sale', 'payment');
