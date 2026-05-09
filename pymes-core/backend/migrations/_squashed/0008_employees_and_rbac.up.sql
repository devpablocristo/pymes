-- 0008_employees_and_rbac.up.sql
-- Empleados, roles, permisos, asignaciones, exchange_rates, scheduler_runs.
--
-- Consolida: pymes-core/0011_transversal_infra (roles, role_permissions,
-- user_roles, exchange_rates, scheduler_runs), 0070_employees, 0074_employees_metadata,
-- 0051_english_role_names.

CREATE TABLE IF NOT EXISTS employees (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    first_name text NOT NULL DEFAULT '',
    last_name text NOT NULL DEFAULT '',
    email text NOT NULL DEFAULT '',
    phone text NOT NULL DEFAULT '',
    position text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active'
        CONSTRAINT employees_status_check
        CHECK (status IN ('active','inactive','terminated')),
    hire_date date,
    end_date date,
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    notes text NOT NULL DEFAULT '',
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_employees_org ON employees(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_employees_org_status ON employees(org_id, status);
CREATE INDEX IF NOT EXISTS idx_employees_org_email
    ON employees(org_id, email) WHERE email <> '';
CREATE INDEX IF NOT EXISTS idx_employees_org_archived_at
    ON employees(org_id, archived_at);

CREATE TRIGGER trg_employees_updated_at
    BEFORE UPDATE ON employees FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    is_system boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT roles_org_name_uniq UNIQUE (org_id, name)
);

CREATE TRIGGER trg_roles_updated_at
    BEFORE UPDATE ON roles FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS role_permissions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    resource text NOT NULL,
    action text NOT NULL,
    CONSTRAINT role_permissions_role_resource_action_uniq
        UNIQUE (role_id, resource, action)
);
CREATE INDEX IF NOT EXISTS idx_role_permissions_role ON role_permissions(role_id);

CREATE TABLE IF NOT EXISTS user_roles (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    assigned_by text,
    assigned_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, org_id)
);
CREATE INDEX IF NOT EXISTS idx_user_roles_org ON user_roles(org_id);

CREATE TABLE IF NOT EXISTS exchange_rates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    from_currency text NOT NULL,
    to_currency text NOT NULL,
    rate_type text NOT NULL
        CONSTRAINT exchange_rates_rate_type_check
        CHECK (rate_type IN ('official','blue','mep','ccl','crypto','custom')),
    buy_rate numeric(15,4) NOT NULL,
    sell_rate numeric(15,4) NOT NULL,
    source text NOT NULL DEFAULT 'manual'
        CONSTRAINT exchange_rates_source_check
        CHECK (source IN ('api','manual')),
    rate_date date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT exchange_rates_org_pair_type_date_uniq
        UNIQUE (org_id, from_currency, to_currency, rate_type, rate_date)
);
CREATE INDEX IF NOT EXISTS idx_exchange_rates_org_date
    ON exchange_rates(org_id, rate_date DESC);
CREATE INDEX IF NOT EXISTS idx_exchange_rates_latest
    ON exchange_rates(org_id, from_currency, to_currency, rate_type, rate_date DESC);

CREATE TABLE IF NOT EXISTS scheduler_runs (
    task_name text PRIMARY KEY,
    last_run_at timestamptz NOT NULL DEFAULT now(),
    next_run_at timestamptz NOT NULL,
    status text NOT NULL DEFAULT 'ok',
    error_message text NOT NULL DEFAULT ''
);
