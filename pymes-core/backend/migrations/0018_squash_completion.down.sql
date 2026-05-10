-- 0018_squash_completion.down.sql

DROP TRIGGER IF EXISTS trg_webhook_events_clerk_updated_at ON webhook_events_clerk;
DROP TABLE IF EXISTS webhook_events_clerk CASCADE;

CREATE TABLE IF NOT EXISTS webhook_events_clerk (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    svix_id text NOT NULL UNIQUE,
    event_type text NOT NULL,
    payload_json jsonb NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now()
);

DROP TRIGGER IF EXISTS trg_tenant_invitations_updated_at ON tenant_invitations;
DROP TABLE IF EXISTS tenant_invitations CASCADE;

CREATE TABLE IF NOT EXISTS tenant_invitations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    email text NOT NULL,
    role text NOT NULL DEFAULT 'member'
        CONSTRAINT tenant_invitations_role_check CHECK (role IN ('admin','member','secops')),
    token text NOT NULL UNIQUE,
    invited_by text,
    accepted_at timestamptz,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE tenant_settings
    DROP COLUMN IF EXISTS onboarding_completed_at,
    DROP COLUMN IF EXISTS vertical,
    DROP COLUMN IF EXISTS payment_method,
    DROP COLUMN IF EXISTS uses_billing,
    DROP COLUMN IF EXISTS client_label,
    DROP COLUMN IF EXISTS sells,
    DROP COLUMN IF EXISTS team_size,
    DROP COLUMN IF EXISTS supported_currencies,
    DROP COLUMN IF EXISTS show_dual_prices,
    DROP COLUMN IF EXISTS auto_fetch_rates,
    DROP COLUMN IF EXISTS default_rate_type,
    DROP COLUMN IF EXISTS secondary_currency,
    DROP COLUMN IF EXISTS scheduling_reminder_hours,
    DROP COLUMN IF EXISTS scheduling_label,
    DROP COLUMN IF EXISTS scheduling_enabled,
    DROP COLUMN IF EXISTS show_qr_in_pdf,
    DROP COLUMN IF EXISTS bank_name,
    DROP COLUMN IF EXISTS bank_alias,
    DROP COLUMN IF EXISTS bank_cbu,
    DROP COLUMN IF EXISTS bank_holder,
    DROP COLUMN IF EXISTS wa_payment_link_template,
    DROP COLUMN IF EXISTS wa_payment_template,
    DROP COLUMN IF EXISTS wa_default_country_code,
    DROP COLUMN IF EXISTS wa_receipt_template,
    DROP COLUMN IF EXISTS wa_quote_template,
    DROP COLUMN IF EXISTS business_email,
    DROP COLUMN IF EXISTS business_phone,
    DROP COLUMN IF EXISTS business_address,
    DROP COLUMN IF EXISTS business_tax_id,
    DROP COLUMN IF EXISTS business_name,
    DROP COLUMN IF EXISTS allow_negative_stock,
    DROP COLUMN IF EXISTS next_credit_note_number,
    DROP COLUMN IF EXISTS next_return_number,
    DROP COLUMN IF EXISTS next_purchase_number,
    DROP COLUMN IF EXISTS next_sale_number,
    DROP COLUMN IF EXISTS next_quote_number,
    DROP COLUMN IF EXISTS credit_note_prefix,
    DROP COLUMN IF EXISTS return_prefix,
    DROP COLUMN IF EXISTS purchase_prefix,
    DROP COLUMN IF EXISTS sale_prefix,
    DROP COLUMN IF EXISTS quote_prefix,
    DROP COLUMN IF EXISTS tax_rate,
    DROP COLUMN IF EXISTS currency,
    DROP COLUMN IF EXISTS hard_limits;

DROP INDEX IF EXISTS idx_org_api_keys_hash;
ALTER TABLE org_api_keys
    DROP COLUMN IF EXISTS rotated_at,
    DROP COLUMN IF EXISTS created_by,
    DROP COLUMN IF EXISTS key_prefix;

DROP TRIGGER IF EXISTS trg_org_members_updated_at ON org_members;
DROP INDEX IF EXISTS idx_org_members_status;
DROP INDEX IF EXISTS idx_org_members_active_user;
DROP INDEX IF EXISTS idx_org_members_one_active_owner;
ALTER TABLE org_members
    DROP CONSTRAINT IF EXISTS org_members_status_check,
    DROP CONSTRAINT IF EXISTS org_members_role_check,
    DROP COLUMN IF EXISTS party_id,
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS removed_at,
    DROP COLUMN IF EXISTS status,
    ADD CONSTRAINT org_members_role_check CHECK (role IN ('admin','member','secops')),
    ADD CONSTRAINT org_members_org_user_uniq UNIQUE (org_id, user_id);

ALTER TABLE users
    DROP COLUMN IF EXISTS phone,
    DROP COLUMN IF EXISTS family_name,
    DROP COLUMN IF EXISTS given_name;

DROP TRIGGER IF EXISTS trg_orgs_updated_at ON orgs;
DROP INDEX IF EXISTS idx_orgs_slug;
DROP INDEX IF EXISTS idx_orgs_clerk_org_id;
ALTER TABLE orgs
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS slug,
    DROP COLUMN IF EXISTS clerk_org_id;
