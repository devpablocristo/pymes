-- 0007_sales.down.sql

DROP TRIGGER IF EXISTS trg_accounts_updated_at ON accounts;
DROP TRIGGER IF EXISTS trg_recurring_expenses_updated_at ON recurring_expenses;
DROP TRIGGER IF EXISTS trg_invoices_updated_at ON invoices;
DROP TRIGGER IF EXISTS trg_purchases_updated_at ON purchases;
DROP TRIGGER IF EXISTS trg_sales_updated_at ON sales;
DROP TRIGGER IF EXISTS trg_quotes_updated_at ON quotes;

DROP TABLE IF EXISTS account_movements;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS recurring_expenses;
DROP TABLE IF EXISTS cash_movements;
DROP TABLE IF EXISTS invoice_line_items;
DROP TABLE IF EXISTS invoices;
DROP TABLE IF EXISTS credit_notes;
DROP TABLE IF EXISTS return_items;
DROP TABLE IF EXISTS returns;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS purchase_items;
DROP TABLE IF EXISTS purchases;
DROP TABLE IF EXISTS sale_items;
DROP TABLE IF EXISTS sales;
DROP TABLE IF EXISTS quote_items;
DROP TABLE IF EXISTS quotes;
