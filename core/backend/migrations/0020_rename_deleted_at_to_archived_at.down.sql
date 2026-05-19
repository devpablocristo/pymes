-- Reverts 0020: archived_at → deleted_at across 12 tables.

BEGIN;

ALTER TABLE IF EXISTS price_lists        RENAME COLUMN archived_at TO deleted_at;
ALTER INDEX IF EXISTS idx_price_lists_org_archived_at        RENAME TO idx_price_lists_org_deleted_at;
ALTER TABLE IF EXISTS employees          RENAME COLUMN archived_at TO deleted_at;
ALTER INDEX IF EXISTS idx_employees_org_archived_at          RENAME TO idx_employees_org_deleted_at;
ALTER TABLE IF EXISTS recurring_expenses RENAME COLUMN archived_at TO deleted_at;
ALTER INDEX IF EXISTS idx_recurring_expenses_org_archived_at RENAME TO idx_recurring_expenses_org_deleted_at;
ALTER TABLE IF EXISTS cash_movements     RENAME COLUMN archived_at TO deleted_at;
ALTER INDEX IF EXISTS idx_cash_movements_org_archived_at     RENAME TO idx_cash_movements_org_deleted_at;
ALTER TABLE IF EXISTS payments           RENAME COLUMN archived_at TO deleted_at;
ALTER INDEX IF EXISTS idx_payments_org_archived_at           RENAME TO idx_payments_org_deleted_at;
ALTER TABLE IF EXISTS returns            RENAME COLUMN archived_at TO deleted_at;
ALTER INDEX IF EXISTS idx_returns_org_archived_at            RENAME TO idx_returns_org_deleted_at;
ALTER TABLE IF EXISTS invoices           RENAME COLUMN archived_at TO deleted_at;
ALTER INDEX IF EXISTS idx_invoices_org_archived_at           RENAME TO idx_invoices_org_deleted_at;
ALTER TABLE IF EXISTS products           RENAME COLUMN archived_at TO deleted_at;
ALTER INDEX IF EXISTS idx_products_org_archived_at           RENAME TO idx_products_org_deleted_at;
ALTER TABLE IF EXISTS services           RENAME COLUMN archived_at TO deleted_at;
ALTER INDEX IF EXISTS idx_services_org_archived_at           RENAME TO idx_services_org_deleted_at;
ALTER TABLE IF EXISTS sales              RENAME COLUMN archived_at TO deleted_at;
ALTER INDEX IF EXISTS idx_sales_org_archived_at              RENAME TO idx_sales_org_deleted_at;
ALTER TABLE IF EXISTS purchases          RENAME COLUMN archived_at TO deleted_at;
ALTER INDEX IF EXISTS idx_purchases_org_archived_at          RENAME TO idx_purchases_org_deleted_at;
ALTER TABLE IF EXISTS parties            RENAME COLUMN archived_at TO deleted_at;
ALTER INDEX IF EXISTS idx_parties_org_archived_at            RENAME TO idx_parties_org_deleted_at;

COMMIT;
