CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS orgs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id text UNIQUE,
    name text NOT NULL,
    slug text UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id text UNIQUE NOT NULL,
    email text UNIQUE NOT NULL,
    name text NOT NULL DEFAULT '',
    avatar_url text NOT NULL DEFAULT '',
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS org_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role text NOT NULL DEFAULT 'member',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_org_members_org ON org_members(org_id);
CREATE INDEX IF NOT EXISTS idx_org_members_user ON org_members(user_id);

CREATE TABLE IF NOT EXISTS tenant_settings (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    plan_code text NOT NULL DEFAULT 'starter',
    hard_limits jsonb NOT NULL DEFAULT '{}',
    updated_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS org_api_keys (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name text NOT NULL DEFAULT '',
    key_hash text UNIQUE NOT NULL,
    key_prefix text NOT NULL DEFAULT '',
    created_by text,
    rotated_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_org_api_keys_org ON org_api_keys(org_id);
CREATE INDEX IF NOT EXISTS idx_org_api_keys_hash ON org_api_keys(key_hash);

CREATE TABLE IF NOT EXISTS org_api_key_scopes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key_id uuid NOT NULL REFERENCES org_api_keys(id) ON DELETE CASCADE,
    scope text NOT NULL,
    UNIQUE(key_id, scope)
);

CREATE TABLE IF NOT EXISTS audit_log (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL,
    actor text,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text,
    payload jsonb,
    prev_hash text,
    hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_org_created ON audit_log(org_id, created_at DESC);

CREATE TABLE IF NOT EXISTS org_usage_counters (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    counter_name text NOT NULL,
    value bigint NOT NULL DEFAULT 0,
    period text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, counter_name, period)
);

CREATE TABLE IF NOT EXISTS admin_activity_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    actor text,
    action text NOT NULL,
    resource_type text NOT NULL DEFAULT '',
    resource_id text,
    payload jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_admin_activity_org ON admin_activity_events(org_id, created_at DESC);
