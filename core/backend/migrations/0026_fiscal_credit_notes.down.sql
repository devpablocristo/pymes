-- 0026_fiscal_credit_notes.down.sql — reverso de 0026.

DROP INDEX IF EXISTS idx_fiscal_vouchers_return;
ALTER TABLE fiscal_vouchers DROP COLUMN IF EXISTS associated_voucher_id;
ALTER TABLE fiscal_vouchers DROP COLUMN IF EXISTS return_id;
