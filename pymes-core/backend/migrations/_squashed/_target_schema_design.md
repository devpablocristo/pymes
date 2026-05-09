# Target Schema Design (post-squash)

> **Fase C** del plan: catálogo declarativo del schema objetivo que el bootstrap nuevo (pymes-core 0001..0017) debe producir desde DB vacía. Fuente para validar `scripts/migrations-validate.sh` cuando termine la Fase D.
>
> No es un `pg_dump` real — el schema actual está corrupto (drift saas vs pymes-core) y un dump del estado bugged sería un mal baseline. En su lugar describe la decisión de diseño tabla por tabla, alineada al plan canónico.
>
> Plan: [`.claude/plans/tengo-un-bug-en-melodic-river.md`](../../../../.claude/plans/tengo-un-bug-en-melodic-river.md). Inventario: [`docs/MIGRATIONS_AUDIT.md`](../../../../docs/MIGRATIONS_AUDIT.md).

## Decisiones globales

- **Identidad**: tabla canónica `orgs` (saas). `tenants` no aparece.
- **Membership**: `org_members`. `tenant_memberships` no aparece.
- **API keys**: `org_api_keys` + `org_api_key_scopes`.
- **Settings**: `tenant_settings (org_id PK)` con todas las columnas saas (stripe, billing_status, status, deleted_at, …).
- **Multi-tenant FK**: `org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE` en TODA tabla operacional.
- **Soft-delete**: `archived_at timestamptz NULL` por convención. Excepción: `users.deleted_at` (anonimización GDPR).
- **Timestamps**: `created_at`/`updated_at` con `DEFAULT now()`. Trigger `set_updated_at()` mantiene `updated_at` automático en cada UPDATE.
- **UUIDs**: `gen_random_uuid()` (de `pgcrypto`). `uuid-ossp` no se usa.
- **`schema_migrations`**: única tabla compartida `(scope, version, applied_at, dirty)` PK `(scope, version)`. Patrón portado de `core/saas/go/migrations`.
- **Naming**: `idx_<tabla>_<cols>` para índices, `<tabla>_<col>_fkey/uniq/check` para constraints. Todos `IF NOT EXISTS` cuando aplica.
- **Transacciones**: cada migración envuelta en `BEGIN; … COMMIT;` salvo la que crea `pgcrypto`/`btree_gist` (que no admiten transacciones en algunos PG).
- **RLS**: out of scope — multi-tenant isolation sigue por filtrado en código Go.

## Inventario de tablas objetivo

### Identity (0001_saas_identity)

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE orgs (
    id uuid PRIMARY KEY,
    name text NOT NULL UNIQUE,
    external_id text,  -- Clerk org id, opcional
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_orgs_external_id ON orgs(external_id) WHERE external_id IS NOT NULL;

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id text NOT NULL UNIQUE,
    email text NOT NULL UNIQUE,
    name text NOT NULL DEFAULT '',
    avatar_url text,                     -- nullable, alineado a saas
    deleted_at timestamptz,              -- GDPR / anonimización (excepción al patrón archived_at)
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_users_external_id ON users(external_id);

CREATE TABLE org_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role text NOT NULL DEFAULT 'member' CHECK (role IN ('admin','member','secops')),
    joined_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, user_id)
);
CREATE INDEX idx_org_members_org_id ON org_members(org_id);
CREATE INDEX idx_org_members_user_id ON org_members(user_id);

CREATE TABLE org_api_keys (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    api_key_hash text NOT NULL UNIQUE,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE org_api_key_scopes (
    id uuid PRIMARY KEY,
    api_key_id uuid NOT NULL REFERENCES org_api_keys(id) ON DELETE CASCADE,
    scope text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_org_api_key_scopes_api_key_id ON org_api_key_scopes(api_key_id);

CREATE TABLE tenant_settings (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    plan_code text NOT NULL DEFAULT 'starter',
    hard_limits_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    stripe_customer_id text UNIQUE,
    stripe_subscription_id text UNIQUE,
    billing_status text NOT NULL DEFAULT 'trialing'
        CHECK (billing_status IN ('trialing','active','past_due','canceled','unpaid')),
    past_due_since timestamptz,
    status text NOT NULL DEFAULT 'active'
        CHECK (status IN ('active','suspended','deleted')),
    deleted_at timestamptz,
    updated_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_tenant_settings_stripe_customer ON tenant_settings(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;
CREATE INDEX idx_tenant_settings_past_due_since ON tenant_settings(past_due_since) WHERE billing_status = 'past_due';

CREATE TABLE org_usage_counters (
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    period date NOT NULL,
    counter text NOT NULL,
    value bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, period, counter)
);

CREATE TABLE saas_usage_event_dedup (
    event_id text PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    counter text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE admin_activity_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    actor text,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text,
    payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_admin_activity_events_org_created ON admin_activity_events(org_id, created_at DESC);
```

### Audit + governance (0002)

```sql
CREATE TABLE audit_log (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    actor text,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text,
    payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    prev_hash text,
    hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_log_org_created ON audit_log(org_id, created_at DESC);

CREATE TABLE protected_resources (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name text NOT NULL,
    resource_type text NOT NULL,
    match_value text NOT NULL,
    match_mode text NOT NULL DEFAULT 'exact',
    environment text NOT NULL DEFAULT '*',
    reason text NOT NULL DEFAULT '',
    enabled boolean NOT NULL DEFAULT true,
    created_by text,
    updated_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_protected_resources_org_created ON protected_resources(org_id, created_at DESC);
CREATE INDEX idx_protected_resources_org_enabled ON protected_resources(org_id, enabled);

CREATE TABLE restore_evidence (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    environment text NOT NULL DEFAULT 'prod',
    system text NOT NULL,
    status text NOT NULL,
    snapshot_id text NOT NULL DEFAULT '',
    restore_target text NOT NULL DEFAULT '',
    started_at timestamptz,
    completed_at timestamptz,
    source text NOT NULL DEFAULT '',
    artifact_sha256 text NOT NULL DEFAULT '',
    summary_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_restore_evidence_org_created ON restore_evidence(org_id, created_at DESC);
CREATE INDEX idx_restore_evidence_org_system_env ON restore_evidence(org_id, system, environment, created_at DESC);

CREATE TABLE tenant_invitations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    email text NOT NULL,
    role text NOT NULL DEFAULT 'member',
    token text NOT NULL UNIQUE,
    invited_by text,
    accepted_at timestamptz,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_tenant_invitations_org ON tenant_invitations(org_id);
CREATE INDEX idx_tenant_invitations_email ON tenant_invitations(lower(email));

CREATE TABLE webhook_events_clerk (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    svix_id text NOT NULL UNIQUE,
    event_type text NOT NULL,
    payload_json jsonb NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    error_message text
);
CREATE INDEX idx_webhook_events_clerk_received ON webhook_events_clerk(received_at DESC);
```

### Notifications (0003)

```sql
CREATE TABLE notification_preferences (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    notification_type text NOT NULL,
    channel text NOT NULL DEFAULT 'email',
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(user_id, notification_type, channel)
);
CREATE INDEX idx_notification_prefs_user ON notification_preferences(user_id);

CREATE TABLE notification_log (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    notification_type text NOT NULL,
    channel text NOT NULL DEFAULT 'email',
    recipient text NOT NULL,
    subject text NOT NULL,
    status text NOT NULL DEFAULT 'sent',
    dedup_key text,
    error_message text,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_notification_log_org_created ON notification_log(org_id, created_at DESC);
CREATE UNIQUE INDEX idx_notification_log_dedup_key ON notification_log(dedup_key) WHERE dedup_key IS NOT NULL;

CREATE TABLE in_app_notifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    actor_id text NOT NULL DEFAULT '',
    type text NOT NULL,
    title text NOT NULL,
    body text NOT NULL DEFAULT '',
    read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_inapp_notif_org_unread ON in_app_notifications(org_id, read_at) WHERE read_at IS NULL;
CREATE INDEX idx_inapp_notif_actor_created ON in_app_notifications(actor_id, created_at DESC);
```

### Party model (0004)

Tablas: `parties`, `party_persons`, `party_organizations`, `party_classifications`, `party_contacts`, `party_agents`. Todas con `org_id NOT NULL REFERENCES orgs(id) ON DELETE CASCADE`, `archived_at` para soft-delete (no `deleted_at`). FK explícitas con `ON DELETE`. Schema basado en `pymes-core/0017_party_model.up.sql` actual, alineado a convenciones.

### Commercial (0005)

Tablas: `products`, `services`, `categories`, `price_lists`, `price_list_items`, `service_price_lists`. Todas con `org_id`, `archived_at`. La separación products/services del 0042/0043 actual ya integrada.

### Inventory (0006)

`stock_levels`, `stock_movements`. Suppliers como `party_role` (no tabla aparte).

### Sales (0007)

`quotes`, `quote_items`, `sales`, `sale_items`, `payments`, `returns`, `return_items`, `credit_notes`, `invoices`, `invoice_line_items`. Todas con `org_id`, `archived_at` o `voided_at` (sales) según corresponda.

### Accounting (0008)

`accounts`, `account_movements`, `cash_movements`, `recurring_expenses`.

### Employees (0009)

`employees`, `roles`, `role_permissions`, `user_roles`. Roles es global (no `org_id`).

### Messaging (0010)

`whatsapp_connections`, `whatsapp_messages`, `whatsapp_templates`, `whatsapp_opt_ins`, `whatsapp_opt_outs`, `whatsapp_campaigns`, `whatsapp_campaign_recipients`, `whatsapp_conversations`. Todas con `org_id`.

### Dashboard (0011)

`dashboard_widgets`, `user_dashboard_layouts`. (Eliminadas legacy: `dashboard_layouts`, `dashboard_default_layouts`, `dashboard_configs`.)

### Calendar (0012)

`calendar_export_tokens`, `calendar_sync_connections`, `calendar_sync_errors`.

### AI (0013)

`ai_dossiers`, `ai_conversations`, `ai_usage_daily`, `ai_agent_events`.

### Agent (0014)

`agent_confirmations`, `agent_idempotency_keys`.

### Attachments + timeline (0015)

`attachments`, `timeline_entries`.

### Webhooks (0016)

`webhook_endpoints`, `webhook_deliveries`, `webhook_outbox`. (`webhook_events_clerk` está en 0002.)

### Business insights + utilidad (0017)

`pymes_business_insight_candidates`, `exchange_rates`, `scheduler_runs`. Cierra el bootstrap.

## Tablas que desaparecen (lista negativa)

Vienen del schema actual y NO deben existir post-squash:

- `tenants`
- `tenant_memberships`
- `tenant_api_keys`
- `tenant_api_key_scopes`
- `tenant_usage_counters`
- `procurement_policies`
- `procurement_requests` *(decidir: si se usa, queda; si no, drop)*
- `procurement_request_items`
- `appointments`
- `appointment_slots`
- `dashboard_layouts`
- `dashboard_default_layouts`
- `dashboard_configs`
- `customers` (rol en `parties`)
- `suppliers` (rol en `parties`)
- `catalog_services` (renamed a `services` ya en 0042/0043 actual; el squash no la recrea)
- `pymes_notification_log` (vuelve a llamarse `notification_log` con schema saas)
- `pymes_notification_preferences` (vuelve a `notification_preferences`)
- `pymes_in_app_notifications` (vuelve a `in_app_notifications`)
- Schema `professionals` con tablas internas legacy (squash las consolidate en migraciones planas en `professionals/backend/migrations`)
- Schema `workshops`, `beauty`, `restaurant` con tablas internas legacy

## Schema de bootstrap tracking

```sql
CREATE TABLE schema_migrations (
    scope text NOT NULL,
    version text NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT now(),
    dirty boolean NOT NULL DEFAULT false,
    PRIMARY KEY (scope, version)
);
```

Scopes esperados al final del bootstrap:
- `pymes-core` (versions: `0001`..`0017`)
- `scheduling` (versions del módulo `modules/scheduling/go`)
- `professionals` (versions del vertical)
- `workshops` (versions del vertical)
- `beauty` (versions del vertical)
- `restaurants` (versions del vertical)

## Function + trigger genérico

```sql
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;
```

Aplicado vía `CREATE TRIGGER trg_<tabla>_updated_at BEFORE UPDATE ON <tabla> FOR EACH ROW EXECUTE FUNCTION set_updated_at();` en cada tabla que tenga `updated_at`.

## Uso

Cuando termine la Fase D (migraciones nuevas escritas):
1. Levantar postgres efímero.
2. Correr `migrations.Run()` desde DB vacía.
3. `pg_dump --schema-only --no-owner` del resultado.
4. Comparar contra este documento (manualmente o vía script de validación que parsea ambos).
5. Diferencias = bugs en las nuevas migraciones, NO en este documento.

Una vez que el bootstrap fresco produce un schema consistente con esta especificación, este documento se vuelve la baseline canónica del repo.
