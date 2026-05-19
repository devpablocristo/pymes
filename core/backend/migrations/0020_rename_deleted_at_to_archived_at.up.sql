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
-- Idempotency: each statement is guarded with IF EXISTS / IF NOT EXISTS where
-- supported by PostgreSQL 11+. Run a second time and it's a no-op.

BEGIN;

-- 1. price_lists
ALTER TABLE IF EXISTS price_lists RENAME COLUMN deleted_at TO archived_at;
ALTER INDEX IF EXISTS idx_price_lists_org_deleted_at RENAME TO idx_price_lists_org_archived_at;

-- 2. employees
ALTER TABLE IF EXISTS employees RENAME COLUMN deleted_at TO archived_at;
ALTER INDEX IF EXISTS idx_employees_org_deleted_at RENAME TO idx_employees_org_archived_at;

-- 3. recurring_expenses
ALTER TABLE IF EXISTS recurring_expenses RENAME COLUMN deleted_at TO archived_at;
ALTER INDEX IF EXISTS idx_recurring_expenses_org_deleted_at RENAME TO idx_recurring_expenses_org_archived_at;

-- 4. cash_movements
ALTER TABLE IF EXISTS cash_movements RENAME COLUMN deleted_at TO archived_at;
ALTER INDEX IF EXISTS idx_cash_movements_org_deleted_at RENAME TO idx_cash_movements_org_archived_at;

-- 5. payments
ALTER TABLE IF EXISTS payments RENAME COLUMN deleted_at TO archived_at;
ALTER INDEX IF EXISTS idx_payments_org_deleted_at RENAME TO idx_payments_org_archived_at;

-- 6. returns
ALTER TABLE IF EXISTS returns RENAME COLUMN deleted_at TO archived_at;
ALTER INDEX IF EXISTS idx_returns_org_deleted_at RENAME TO idx_returns_org_archived_at;

-- 7. invoices
ALTER TABLE IF EXISTS invoices RENAME COLUMN deleted_at TO archived_at;
ALTER INDEX IF EXISTS idx_invoices_org_deleted_at RENAME TO idx_invoices_org_archived_at;

-- 8. products
ALTER TABLE IF EXISTS products RENAME COLUMN deleted_at TO archived_at;
ALTER INDEX IF EXISTS idx_products_org_deleted_at RENAME TO idx_products_org_archived_at;

-- 9. services
ALTER TABLE IF EXISTS services RENAME COLUMN deleted_at TO archived_at;
ALTER INDEX IF EXISTS idx_services_org_deleted_at RENAME TO idx_services_org_archived_at;

-- 10. sales
ALTER TABLE IF EXISTS sales RENAME COLUMN deleted_at TO archived_at;
ALTER INDEX IF EXISTS idx_sales_org_deleted_at RENAME TO idx_sales_org_archived_at;

-- 11. purchases
ALTER TABLE IF EXISTS purchases RENAME COLUMN deleted_at TO archived_at;
ALTER INDEX IF EXISTS idx_purchases_org_deleted_at RENAME TO idx_purchases_org_archived_at;

-- 12. parties (customers + suppliers share this table)
ALTER TABLE IF EXISTS parties RENAME COLUMN deleted_at TO archived_at;
ALTER INDEX IF EXISTS idx_parties_org_deleted_at RENAME TO idx_parties_org_archived_at;

COMMIT;
