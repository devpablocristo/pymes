ALTER TABLE tenant_settings
    DROP COLUMN IF EXISTS wa_payment_link_template,
    DROP COLUMN IF EXISTS wa_payment_template,
    DROP COLUMN IF EXISTS show_qr_in_pdf,
    DROP COLUMN IF EXISTS bank_name,
    DROP COLUMN IF EXISTS bank_alias,
    DROP COLUMN IF EXISTS bank_cbu,
    DROP COLUMN IF EXISTS bank_holder;

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_method_check;
ALTER TABLE payments ADD CONSTRAINT payments_method_check
    CHECK (method IN ('cash', 'card', 'transfer', 'check', 'other', 'credit_note'));

DROP TABLE IF EXISTS payment_preferences;
DROP TABLE IF EXISTS payment_gateway_connections;
