ALTER TABLE tenant_settings
    DROP COLUMN IF EXISTS appointment_reminder_hours,
    DROP COLUMN IF EXISTS appointment_label,
    DROP COLUMN IF EXISTS appointments_enabled,

    DROP COLUMN IF EXISTS wa_default_country_code,
    DROP COLUMN IF EXISTS wa_receipt_template,
    DROP COLUMN IF EXISTS wa_quote_template,

    DROP COLUMN IF EXISTS business_email,
    DROP COLUMN IF EXISTS business_phone,
    DROP COLUMN IF EXISTS business_address,
    DROP COLUMN IF EXISTS business_tax_id,
    DROP COLUMN IF EXISTS business_name,

    DROP COLUMN IF EXISTS next_credit_note_number,
    DROP COLUMN IF EXISTS next_return_number,
    DROP COLUMN IF EXISTS credit_note_prefix,
    DROP COLUMN IF EXISTS return_prefix,

    DROP COLUMN IF EXISTS next_purchase_number,
    DROP COLUMN IF EXISTS purchase_prefix;
