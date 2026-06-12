-- Ola C step 10 (final) — Align CRUDAR column naming across pymes-core.
--
-- The lifecycle.Service refactor (Olas B0 + C1-9) standardizes the soft-delete
-- column as `archived_at` (sec. 5.5 of CLAUDE.md + the platform/lifecycle/go
-- contract). Quotes was renamed in 0019; this migration brings the remaining
-- 12 tables in line.
--
-- Tables touched (all owned by pymes-core/backend/internal/*):
--   price_lists, employees, recurring_expenses, cash_movements, payments,
--   returns, invoices, products, services, sales, purchases, parties.
--
-- Each table:
--   1. ALTER TABLE ... RENAME COLUMN deleted_at TO archived_at;
--   2. ALTER INDEX ... RENAME (when an index exists on the column).
--
-- Idempotency: PostgreSQL has ALTER TABLE IF EXISTS, but not RENAME COLUMN IF
-- EXISTS. The DO block below checks both source and destination columns before
-- renaming so mixed local schemas are safe.

BEGIN;

DO $$
DECLARE
  item record;
BEGIN
  FOR item IN
    SELECT * FROM (VALUES
      ('price_lists',        'idx_price_lists_org_deleted_at',        'idx_price_lists_org_archived_at'),
      ('employees',          'idx_employees_org_deleted_at',          'idx_employees_org_archived_at'),
      ('recurring_expenses', 'idx_recurring_expenses_org_deleted_at', 'idx_recurring_expenses_org_archived_at'),
      ('cash_movements',     'idx_cash_movements_org_deleted_at',     'idx_cash_movements_org_archived_at'),
      ('payments',           'idx_payments_org_deleted_at',           'idx_payments_org_archived_at'),
      ('returns',            'idx_returns_org_deleted_at',            'idx_returns_org_archived_at'),
      ('invoices',           'idx_invoices_org_deleted_at',           'idx_invoices_org_archived_at'),
      ('products',           'idx_products_org_deleted_at',           'idx_products_org_archived_at'),
      ('services',           'idx_services_org_deleted_at',           'idx_services_org_archived_at'),
      ('sales',              'idx_sales_org_deleted_at',              'idx_sales_org_archived_at'),
      ('purchases',          'idx_purchases_org_deleted_at',          'idx_purchases_org_archived_at'),
      ('parties',            'idx_parties_org_deleted_at',            'idx_parties_org_archived_at')
    ) AS rename_plan(table_name, old_index_name, new_index_name)
  LOOP
    IF to_regclass(format('public.%I', item.table_name)) IS NOT NULL
      AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = item.table_name
          AND column_name = 'deleted_at'
      )
      AND NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = item.table_name
          AND column_name = 'archived_at'
      )
    THEN
      EXECUTE format('ALTER TABLE %I RENAME COLUMN deleted_at TO archived_at', item.table_name);
    END IF;

    IF to_regclass(format('public.%I', item.old_index_name)) IS NOT NULL
      AND to_regclass(format('public.%I', item.new_index_name)) IS NULL
    THEN
      EXECUTE format('ALTER INDEX %I RENAME TO %I', item.old_index_name, item.new_index_name);
    END IF;
  END LOOP;
END $$;

COMMIT;
