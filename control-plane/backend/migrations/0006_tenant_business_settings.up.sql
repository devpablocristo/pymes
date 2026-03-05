ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS currency text NOT NULL DEFAULT 'ARS',
    ADD COLUMN IF NOT EXISTS tax_rate numeric(5,2) NOT NULL DEFAULT 21.00,
    ADD COLUMN IF NOT EXISTS quote_prefix text NOT NULL DEFAULT 'PRE',
    ADD COLUMN IF NOT EXISTS sale_prefix text NOT NULL DEFAULT 'VTA',
    ADD COLUMN IF NOT EXISTS next_quote_number int NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS next_sale_number int NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS allow_negative_stock boolean NOT NULL DEFAULT true;
