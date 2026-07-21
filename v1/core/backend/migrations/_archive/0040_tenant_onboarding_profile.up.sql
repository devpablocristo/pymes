ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS team_size text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sells text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS client_label text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS uses_billing boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS payment_method text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS vertical text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS onboarding_completed_at timestamptz;
