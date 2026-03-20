CREATE TABLE IF NOT EXISTS roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    is_system boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, name)
);

CREATE TABLE IF NOT EXISTS role_permissions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    resource text NOT NULL,
    action text NOT NULL,
    UNIQUE(role_id, resource, action)
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

CREATE TABLE IF NOT EXISTS attachments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    attachable_type text NOT NULL,
    attachable_id uuid NOT NULL,
    file_name text NOT NULL,
    content_type text NOT NULL DEFAULT 'application/octet-stream',
    size_bytes bigint NOT NULL DEFAULT 0,
    storage_key text NOT NULL,
    uploaded_by text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_attachments_entity ON attachments(org_id, attachable_type, attachable_id);
CREATE INDEX IF NOT EXISTS idx_attachments_org ON attachments(org_id, created_at DESC);

CREATE TABLE IF NOT EXISTS timeline_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
    event_type text NOT NULL,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    actor text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_timeline_entity ON timeline_entries(org_id, entity_type, entity_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_timeline_org ON timeline_entries(org_id, created_at DESC);

CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    url text NOT NULL,
    secret text NOT NULL,
    events text[] NOT NULL DEFAULT '{}',
    is_active boolean NOT NULL DEFAULT true,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_org ON webhook_endpoints(org_id) WHERE is_active = true;

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id uuid NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    status_code int,
    response_body text NOT NULL DEFAULT '',
    attempts int NOT NULL DEFAULT 0,
    next_retry timestamptz,
    delivered_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_endpoint ON webhook_deliveries(endpoint_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_retry ON webhook_deliveries(next_retry) WHERE delivered_at IS NULL AND attempts < 5;

CREATE TABLE IF NOT EXISTS exchange_rates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    from_currency text NOT NULL,
    to_currency text NOT NULL,
    rate_type text NOT NULL CHECK (rate_type IN ('official', 'blue', 'mep', 'ccl', 'crypto', 'custom')),
    buy_rate numeric(15,4) NOT NULL,
    sell_rate numeric(15,4) NOT NULL,
    source text NOT NULL DEFAULT 'manual' CHECK (source IN ('api', 'manual')),
    rate_date date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, from_currency, to_currency, rate_type, rate_date)
);

CREATE INDEX IF NOT EXISTS idx_exchange_rates_org_date ON exchange_rates(org_id, rate_date DESC);
CREATE INDEX IF NOT EXISTS idx_exchange_rates_latest ON exchange_rates(org_id, from_currency, to_currency, rate_type, rate_date DESC);

CREATE TABLE IF NOT EXISTS dashboard_configs (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    widgets jsonb NOT NULL DEFAULT '["sales_today","sales_month","cashflow_balance","pending_quotes","low_stock_products","top_products_month","recent_sales"]'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS scheduler_runs (
    task_name text PRIMARY KEY,
    last_run_at timestamptz NOT NULL DEFAULT now(),
    next_run_at timestamptz NOT NULL,
    status text NOT NULL DEFAULT 'ok',
    error_message text NOT NULL DEFAULT ''
);
