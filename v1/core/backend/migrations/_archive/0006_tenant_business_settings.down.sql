ALTER TABLE tenant_settings
    DROP COLUMN IF EXISTS allow_negative_stock,
    DROP COLUMN IF EXISTS next_sale_number,
    DROP COLUMN IF EXISTS next_quote_number,
    DROP COLUMN IF EXISTS sale_prefix,
    DROP COLUMN IF EXISTS quote_prefix,
    DROP COLUMN IF EXISTS tax_rate,
    DROP COLUMN IF EXISTS currency;
