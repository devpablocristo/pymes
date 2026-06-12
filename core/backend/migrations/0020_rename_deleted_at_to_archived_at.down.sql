-- Reverts 0020: archived_at → deleted_at across 12 tables.

BEGIN;

DO $$
DECLARE
  item record;
BEGIN
  FOR item IN
    SELECT * FROM (VALUES
      ('price_lists',        'idx_price_lists_org_archived_at',        'idx_price_lists_org_deleted_at'),
      ('employees',          'idx_employees_org_archived_at',          'idx_employees_org_deleted_at'),
      ('recurring_expenses', 'idx_recurring_expenses_org_archived_at', 'idx_recurring_expenses_org_deleted_at'),
      ('cash_movements',     'idx_cash_movements_org_archived_at',     'idx_cash_movements_org_deleted_at'),
      ('payments',           'idx_payments_org_archived_at',           'idx_payments_org_deleted_at'),
      ('returns',            'idx_returns_org_archived_at',            'idx_returns_org_deleted_at'),
      ('invoices',           'idx_invoices_org_archived_at',           'idx_invoices_org_deleted_at'),
      ('products',           'idx_products_org_archived_at',           'idx_products_org_deleted_at'),
      ('services',           'idx_services_org_archived_at',           'idx_services_org_deleted_at'),
      ('sales',              'idx_sales_org_archived_at',              'idx_sales_org_deleted_at'),
      ('purchases',          'idx_purchases_org_archived_at',          'idx_purchases_org_deleted_at'),
      ('parties',            'idx_parties_org_archived_at',            'idx_parties_org_deleted_at')
    ) AS rename_plan(table_name, old_index_name, new_index_name)
  LOOP
    IF to_regclass(format('public.%I', item.table_name)) IS NOT NULL
      AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = item.table_name
          AND column_name = 'archived_at'
      )
      AND NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = item.table_name
          AND column_name = 'deleted_at'
      )
    THEN
      EXECUTE format('ALTER TABLE %I RENAME COLUMN archived_at TO deleted_at', item.table_name);
    END IF;

    IF to_regclass(format('public.%I', item.old_index_name)) IS NOT NULL
      AND to_regclass(format('public.%I', item.new_index_name)) IS NULL
    THEN
      EXECUTE format('ALTER INDEX %I RENAME TO %I', item.old_index_name, item.new_index_name);
    END IF;
  END LOOP;
END $$;

COMMIT;
