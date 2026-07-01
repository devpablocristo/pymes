-- 0001_saas_identity.up.sql
-- Identidad SaaS canónica: orgs, users, org_members, org_api_keys + scopes,
-- org_settings (org_id PK), org_usage_counters, saas_usage_event_dedup,
-- admin_activity_events.
--
-- Reemplaza el schema legacy "tenants/tenant_memberships/tenant_api_keys/..."
-- de core/0001_base_schema (squashed). Tablas y schemas alineados
-- con core/saas/go (versionado al squash; core es self-contained
-- post-squash, no depende de saasmigrations.MigrateUp en runtime).
--
-- Convenciones aplicadas:
-- - PK uuid con gen_random_uuid() (pgcrypto) o uuid_generate_v5() (uuid-ossp para seeds).
-- - org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE en multi-tenant.
-- - timestamptz NOT NULL DEFAULT now() para created_at / updated_at.
-- - CHECK constraints para enums (status, role, billing_status).
-- - Índices nombrados explícitamente (idx_<tabla>_<cols>).
-- - users.deleted_at: excepción documentada al patrón archived_at (anonimización GDPR).
-- - Trigger genérico set_updated_at() aplicado a tablas con updated_at.

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Trigger genérico para mantener updated_at automático.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE TABLE IF NOT EXISTS orgs (
    id uuid PRIMARY KEY,
    name text NOT NULL UNIQUE,
    external_id text,
    clerk_org_id text,
    slug text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_orgs_external_id
    ON orgs(external_id) WHERE external_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_orgs_clerk_org_id
    ON orgs(clerk_org_id) WHERE clerk_org_id IS NOT NULL AND clerk_org_id <> '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_orgs_slug
    ON orgs(slug) WHERE slug IS NOT NULL AND slug <> '';

CREATE TRIGGER trg_orgs_updated_at
    BEFORE UPDATE ON orgs FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id text NOT NULL UNIQUE,
    email text NOT NULL UNIQUE,
    name text NOT NULL DEFAULT '',
    given_name text NOT NULL DEFAULT '',
    family_name text NOT NULL DEFAULT '',
    phone text NOT NULL DEFAULT '',
    avatar_url text,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_users_external_id ON users(external_id);

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS org_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    party_id uuid,
    role text NOT NULL DEFAULT 'member'
        CONSTRAINT org_members_role_check CHECK (role IN ('owner','admin','member')),
    status text NOT NULL DEFAULT 'active'
        CONSTRAINT org_members_status_check CHECK (status IN ('active','removed')),
    removed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_org_members_org_id ON org_members(org_id);
CREATE INDEX IF NOT EXISTS idx_org_members_user_id ON org_members(user_id);
CREATE INDEX IF NOT EXISTS idx_org_members_status ON org_members(org_id, status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_org_members_one_active_owner
    ON org_members(org_id) WHERE role = 'owner' AND status = 'active';
CREATE UNIQUE INDEX IF NOT EXISTS idx_org_members_active_user
    ON org_members(org_id, user_id) WHERE status = 'active';

CREATE TRIGGER trg_org_members_updated_at
    BEFORE UPDATE ON org_members FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS org_api_keys (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    api_key_hash text NOT NULL UNIQUE,
    name text NOT NULL,
    key_prefix text NOT NULL DEFAULT '',
    created_by text,
    rotated_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_org_api_keys_org_id ON org_api_keys(org_id);
CREATE INDEX IF NOT EXISTS idx_org_api_keys_hash ON org_api_keys(api_key_hash);

CREATE TABLE IF NOT EXISTS org_api_key_scopes (
    id uuid PRIMARY KEY,
    api_key_id uuid NOT NULL REFERENCES org_api_keys(id) ON DELETE CASCADE,
    scope text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT org_api_key_scopes_key_scope_uniq UNIQUE (api_key_id, scope)
);
CREATE INDEX IF NOT EXISTS idx_org_api_key_scopes_api_key_id
    ON org_api_key_scopes(api_key_id);

CREATE TABLE IF NOT EXISTS org_settings (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    plan_code text NOT NULL DEFAULT 'starter',
    hard_limits jsonb NOT NULL DEFAULT '{}'::jsonb,
    hard_limits_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    stripe_customer_id text UNIQUE,
    stripe_subscription_id text UNIQUE,
    billing_status text NOT NULL DEFAULT 'trialing'
        CONSTRAINT org_settings_billing_status_check
        CHECK (billing_status IN ('trialing','active','past_due','canceled','unpaid')),
    past_due_since timestamptz,
    status text NOT NULL DEFAULT 'active'
        CONSTRAINT org_settings_status_check
        CHECK (status IN ('active','suspended','deleted')),
    deleted_at timestamptz,
    updated_by text,

    -- Comerciales (legacy 0005, 0010, 0064-0068).
    currency text NOT NULL DEFAULT 'ARS',
    tax_rate numeric(5,2) NOT NULL DEFAULT 21.00,
    quote_prefix text NOT NULL DEFAULT 'PRE',
    sale_prefix text NOT NULL DEFAULT 'VTA',
    purchase_prefix text NOT NULL DEFAULT 'CPA',
    return_prefix text NOT NULL DEFAULT 'DEV',
    credit_note_prefix text NOT NULL DEFAULT 'NC',
    next_quote_number int NOT NULL DEFAULT 1,
    next_sale_number int NOT NULL DEFAULT 1,
    next_purchase_number int NOT NULL DEFAULT 1,
    next_return_number int NOT NULL DEFAULT 1,
    next_credit_note_number int NOT NULL DEFAULT 1,
    allow_negative_stock boolean NOT NULL DEFAULT true,

    -- Datos de negocio (legacy 0012).
    business_name text NOT NULL DEFAULT '',
    business_tax_id text NOT NULL DEFAULT '',
    business_address text NOT NULL DEFAULT '',
    business_phone text NOT NULL DEFAULT '',
    business_email text NOT NULL DEFAULT '',

    -- WhatsApp templates (legacy 0012, 0024, 0066).
    wa_quote_template text NOT NULL DEFAULT 'Hola {customer_name}, te enviamos el presupuesto {number} por {total}.',
    wa_receipt_template text NOT NULL DEFAULT 'Hola {customer_name}, tu comprobante de compra {number} por {total}. Gracias por tu compra!',
    wa_default_country_code text NOT NULL DEFAULT '54',
    wa_payment_template text NOT NULL DEFAULT 'Podes transferir a:\nAlias: {bank_alias}\nCBU: {bank_cbu}\nTitular: {bank_holder}\nBanco: {bank_name}\nMonto: {total}',
    wa_payment_link_template text NOT NULL DEFAULT 'Hola {customer_name}, podes pagar {total} de tu compra {number} con este link: {payment_url}',

    -- Datos bancarios (legacy 0066).
    bank_holder text NOT NULL DEFAULT '',
    bank_cbu text NOT NULL DEFAULT '',
    bank_alias text NOT NULL DEFAULT '',
    bank_name text NOT NULL DEFAULT '',
    show_qr_in_pdf boolean NOT NULL DEFAULT false,

    -- Scheduling (legacy 0033, 0041).
    scheduling_enabled boolean NOT NULL DEFAULT false,
    scheduling_label text NOT NULL DEFAULT 'Turno',
    scheduling_reminder_hours int NOT NULL DEFAULT 24,

    -- Multi-currency (legacy 0029).
    secondary_currency text NOT NULL DEFAULT '',
    default_rate_type text NOT NULL DEFAULT 'blue',
    auto_fetch_rates boolean NOT NULL DEFAULT false,
    show_dual_prices boolean NOT NULL DEFAULT false,
    supported_currencies jsonb NOT NULL DEFAULT '[]'::jsonb,

    -- Onboarding profile (legacy 0040).
    team_size text NOT NULL DEFAULT '',
    sells text NOT NULL DEFAULT '',
    client_label text NOT NULL DEFAULT '',
    uses_billing boolean NOT NULL DEFAULT false,
    payment_method text NOT NULL DEFAULT '',
    vertical text NOT NULL DEFAULT '',
    onboarding_completed_at timestamptz,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_org_settings_stripe_customer
    ON org_settings(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_org_settings_past_due_since
    ON org_settings(past_due_since) WHERE billing_status = 'past_due';

CREATE TRIGGER trg_org_settings_updated_at
    BEFORE UPDATE ON org_settings FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS org_usage_counters (
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    period date NOT NULL,
    counter text NOT NULL,
    value bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, period, counter)
);

CREATE TRIGGER trg_org_usage_counters_updated_at
    BEFORE UPDATE ON org_usage_counters FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS saas_usage_event_dedup (
    event_id text PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    counter text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_saas_usage_event_dedup_org
    ON saas_usage_event_dedup(org_id);

CREATE TABLE IF NOT EXISTS admin_activity_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    actor text,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text,
    payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_admin_activity_events_org_created
    ON admin_activity_events(org_id, created_at DESC);
