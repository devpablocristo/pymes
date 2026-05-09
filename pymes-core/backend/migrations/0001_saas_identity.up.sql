-- 0001_saas_identity.up.sql
-- Identidad SaaS canónica: orgs, users, org_members, org_api_keys + scopes,
-- tenant_settings (org_id PK), org_usage_counters, saas_usage_event_dedup,
-- admin_activity_events.
--
-- Reemplaza el schema legacy "tenants/tenant_memberships/tenant_api_keys/..."
-- de pymes-core/0001_base_schema (squashed). Tablas y schemas alineados
-- con core/saas/go (versionado al squash; pymes-core es self-contained
-- post-squash, no depende de saasmigrations.MigrateUp en runtime).
--
-- Convenciones aplicadas:
-- - PK uuid con gen_random_uuid() (pgcrypto).
-- - org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE en multi-tenant.
-- - timestamptz NOT NULL DEFAULT now() para created_at / updated_at.
-- - CHECK constraints para enums (status, role, billing_status).
-- - Índices nombrados explícitamente (idx_<tabla>_<cols>).
-- - users.deleted_at: excepción documentada al patrón archived_at (anonimización GDPR).

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS orgs (
    id uuid PRIMARY KEY,
    name text NOT NULL UNIQUE,
    external_id text,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_orgs_external_id
    ON orgs(external_id) WHERE external_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id text NOT NULL UNIQUE,
    email text NOT NULL UNIQUE,
    name text NOT NULL DEFAULT '',
    avatar_url text,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_users_external_id ON users(external_id);

CREATE TABLE IF NOT EXISTS org_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role text NOT NULL DEFAULT 'member'
        CONSTRAINT org_members_role_check CHECK (role IN ('admin','member','secops')),
    joined_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT org_members_org_user_uniq UNIQUE (org_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_org_members_org_id ON org_members(org_id);
CREATE INDEX IF NOT EXISTS idx_org_members_user_id ON org_members(user_id);

CREATE TABLE IF NOT EXISTS org_api_keys (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    api_key_hash text NOT NULL UNIQUE,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_org_api_keys_org_id ON org_api_keys(org_id);

CREATE TABLE IF NOT EXISTS org_api_key_scopes (
    id uuid PRIMARY KEY,
    api_key_id uuid NOT NULL REFERENCES org_api_keys(id) ON DELETE CASCADE,
    scope text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT org_api_key_scopes_key_scope_uniq UNIQUE (api_key_id, scope)
);
CREATE INDEX IF NOT EXISTS idx_org_api_key_scopes_api_key_id
    ON org_api_key_scopes(api_key_id);

CREATE TABLE IF NOT EXISTS tenant_settings (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    plan_code text NOT NULL DEFAULT 'starter',
    hard_limits_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    stripe_customer_id text UNIQUE,
    stripe_subscription_id text UNIQUE,
    billing_status text NOT NULL DEFAULT 'trialing'
        CONSTRAINT tenant_settings_billing_status_check
        CHECK (billing_status IN ('trialing','active','past_due','canceled','unpaid')),
    past_due_since timestamptz,
    status text NOT NULL DEFAULT 'active'
        CONSTRAINT tenant_settings_status_check
        CHECK (status IN ('active','suspended','deleted')),
    deleted_at timestamptz,
    updated_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tenant_settings_stripe_customer
    ON tenant_settings(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tenant_settings_past_due_since
    ON tenant_settings(past_due_since) WHERE billing_status = 'past_due';

CREATE TABLE IF NOT EXISTS org_usage_counters (
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    period date NOT NULL,
    counter text NOT NULL,
    value bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, period, counter)
);

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

-- Trigger genérico para mantener updated_at automático en cada UPDATE.
-- Se aplica a las tablas con columna updated_at en este archivo y en los siguientes.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_tenant_settings_updated_at
    BEFORE UPDATE ON tenant_settings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_org_usage_counters_updated_at
    BEFORE UPDATE ON org_usage_counters
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
