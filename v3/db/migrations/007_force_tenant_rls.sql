BEGIN;

-- Reassert FORCE for databases that applied early v3 migrations before every
-- tenant table was hardened. API roles must never inherit the owner bypass;
-- the dedicated worker role is the only cross-tenant runtime principal.
ALTER TABLE app.parties FORCE ROW LEVEL SECURITY;
ALTER TABLE app.sales FORCE ROW LEVEL SECURITY;
ALTER TABLE app.idempotency_records FORCE ROW LEVEL SECURITY;
ALTER TABLE app.outbox FORCE ROW LEVEL SECURITY;
ALTER TABLE app.memberships FORCE ROW LEVEL SECURITY;
ALTER TABLE app.purchases FORCE ROW LEVEL SECURITY;
ALTER TABLE app.payments FORCE ROW LEVEL SECURITY;
ALTER TABLE app.open_item_applications FORCE ROW LEVEL SECURITY;
ALTER TABLE app.accounting_application_commands FORCE ROW LEVEL SECURITY;
ALTER TABLE app.accounting_reversals FORCE ROW LEVEL SECURITY;
ALTER TABLE app.fiscal_number_sequences FORCE ROW LEVEL SECURITY;
ALTER TABLE app.outbox_dead_letters FORCE ROW LEVEL SECURITY;

COMMIT;
