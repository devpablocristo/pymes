-- 0026_fiscal_credit_notes.up.sql
-- Notas de crédito/débito: el comprobante fiscal puede originarse en una
-- devolución (return_id) y referenciar el comprobante original (CbtesAsoc).

ALTER TABLE fiscal_vouchers ADD COLUMN IF NOT EXISTS return_id uuid;
ALTER TABLE fiscal_vouchers ADD COLUMN IF NOT EXISTS associated_voucher_id uuid;

CREATE INDEX IF NOT EXISTS idx_fiscal_vouchers_return
    ON fiscal_vouchers(org_id, return_id) WHERE return_id IS NOT NULL;
