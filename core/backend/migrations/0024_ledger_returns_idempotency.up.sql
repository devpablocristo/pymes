-- 0024_ledger_returns_idempotency.up.sql
-- Las devoluciones (returns) son once-only (se crean y se anulan una vez), así
-- que entran al scope de la uq de idempotencia junto a ventas y cobros.

DROP INDEX IF EXISTS uq_journal_entries_idempotency;
CREATE UNIQUE INDEX IF NOT EXISTS uq_journal_entries_idempotency
    ON journal_entries(org_id, source_type, source_id, source_event)
    WHERE source_id IS NOT NULL AND source_type IN ('sale', 'payment', 'return');
