ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS purchase_prefix text NOT NULL DEFAULT 'CPA',
    ADD COLUMN IF NOT EXISTS next_purchase_number int NOT NULL DEFAULT 1,

    ADD COLUMN IF NOT EXISTS return_prefix text NOT NULL DEFAULT 'DEV',
    ADD COLUMN IF NOT EXISTS credit_note_prefix text NOT NULL DEFAULT 'NC',
    ADD COLUMN IF NOT EXISTS next_return_number int NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS next_credit_note_number int NOT NULL DEFAULT 1,

    ADD COLUMN IF NOT EXISTS business_name text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS business_tax_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS business_address text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS business_phone text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS business_email text NOT NULL DEFAULT '',

    ADD COLUMN IF NOT EXISTS wa_quote_template text NOT NULL DEFAULT 'Hola {customer_name}, te enviamos el presupuesto {number} por {total}.',
    ADD COLUMN IF NOT EXISTS wa_receipt_template text NOT NULL DEFAULT 'Hola {customer_name}, tu comprobante de compra {number} por {total}. Gracias por tu compra!',
    ADD COLUMN IF NOT EXISTS wa_default_country_code text NOT NULL DEFAULT '54',

    ADD COLUMN IF NOT EXISTS appointments_enabled boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS appointment_label text NOT NULL DEFAULT 'Turno',
    ADD COLUMN IF NOT EXISTS appointment_reminder_hours int NOT NULL DEFAULT 24,

    ADD COLUMN IF NOT EXISTS secondary_currency text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS default_rate_type text NOT NULL DEFAULT 'blue',
    ADD COLUMN IF NOT EXISTS auto_fetch_rates boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS show_dual_prices boolean NOT NULL DEFAULT false;
