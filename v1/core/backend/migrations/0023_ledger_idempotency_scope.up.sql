-- 0023_ledger_idempotency_scope.up.sql
-- La idempotencia "un asiento por (doc, evento)" sólo aplica a documentos
-- once-only (ventas, cobros). Las compras pueden re-postear su alta tras un
-- storno (toggle received<->draft), así que se excluyen de la uq: su
-- idempotencia la garantiza el worker (reconciliación al estado actual) bajo el
-- lock de la fila del outbox.

DROP INDEX IF EXISTS uq_journal_entries_idempotency;
CREATE UNIQUE INDEX IF NOT EXISTS uq_journal_entries_idempotency
    ON journal_entries(org_id, source_type, source_id, source_event)
    WHERE source_id IS NOT NULL AND source_type IN ('sale', 'payment');
