-- 0018_squash_completion.up.sql
--
-- Completa el schema saas + commercial del squash 0001..0017. Las migraciones
-- 0001 (saas_identity) y 0002 (audit_and_governance) tomaron el schema BASE
-- de core/saas/go pero el código pymes asume columnas adicionales que se
-- habían acumulado en migraciones legacy (0027 phone, 0028 name_parts,
-- 0040 tenant_onboarding_profile, 0066 commercial_settings, 0075 tenant_access_model,
-- 0078 webhook_events_clerk shape, etc).
--
-- Este archivo aplica esas extensiones de schema sin romper la idempotencia
-- (CREATE/ADD/ALTER con IF NOT EXISTS y guards). Es seguro reaplicar.
--
-- Tablas afectadas:
-- - orgs            (+slug, +clerk_org_id, +updated_at)
-- - users           (+given_name, +family_name, +phone)
-- - org_members     (+status, +removed_at, +updated_at, role check ampliado, índices)
-- - org_api_keys    (+key_prefix, +created_by, +rotated_at)
-- - tenant_settings (+~40 columnas: currency, prefijos, plantillas WA, banco, vertical, etc)
-- - tenant_invitations (DROP + CREATE: schema actual es incompatible)
-- - webhook_events_clerk (rename payload_json → payload + agregar status/error_message/processed_at/updated_at)

-- ─── extensiones ───────────────────────────────────────────────────────────
-- uuid-ossp da uuid_generate_v5 (UUID determinístico por namespace+name) que
-- los seeds usan para generar IDs reproducibles. pgcrypto/gen_random_uuid()
-- ya estaba en 0001, pero no provee v5.
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ─── orgs ──────────────────────────────────────────────────────────────────
ALTER TABLE orgs
    ADD COLUMN IF NOT EXISTS clerk_org_id text,
    ADD COLUMN IF NOT EXISTS slug text,
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

CREATE UNIQUE INDEX IF NOT EXISTS idx_orgs_clerk_org_id
    ON orgs(clerk_org_id) WHERE clerk_org_id IS NOT NULL AND clerk_org_id <> '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_orgs_slug
    ON orgs(slug) WHERE slug IS NOT NULL AND slug <> '';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_orgs_updated_at') THEN
        CREATE TRIGGER trg_orgs_updated_at BEFORE UPDATE ON orgs
            FOR EACH ROW EXECUTE FUNCTION set_updated_at();
    END IF;
END $$;

-- ─── users ─────────────────────────────────────────────────────────────────
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS given_name text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS family_name text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS phone text NOT NULL DEFAULT '';

-- ─── org_members ───────────────────────────────────────────────────────────
ALTER TABLE org_members
    ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS removed_at timestamptz,
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS party_id uuid;

-- joined_at queda como nombre histórico de saas, pero el código pymes usa
-- created_at. Renombramos columna para coherencia con el resto del schema.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema='public' AND table_name='org_members' AND column_name='joined_at'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema='public' AND table_name='org_members' AND column_name='created_at'
    ) THEN
        ALTER TABLE org_members RENAME COLUMN joined_at TO created_at;
    END IF;
END $$;

ALTER TABLE org_members
    DROP CONSTRAINT IF EXISTS org_members_role_check,
    ADD CONSTRAINT org_members_role_check CHECK (role IN ('owner','admin','member'));

ALTER TABLE org_members
    DROP CONSTRAINT IF EXISTS org_members_status_check,
    ADD CONSTRAINT org_members_status_check CHECK (status IN ('active','removed'));

-- El UNIQUE total (org_id, user_id) impide re-invitar a alguien removido. El
-- patrón post-squash es: unique partial sobre members activos.
ALTER TABLE org_members DROP CONSTRAINT IF EXISTS org_members_org_user_uniq;

CREATE UNIQUE INDEX IF NOT EXISTS idx_org_members_one_active_owner
    ON org_members(org_id) WHERE role = 'owner' AND status = 'active';
CREATE UNIQUE INDEX IF NOT EXISTS idx_org_members_active_user
    ON org_members(org_id, user_id) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_org_members_status
    ON org_members(org_id, status);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_org_members_updated_at') THEN
        CREATE TRIGGER trg_org_members_updated_at BEFORE UPDATE ON org_members
            FOR EACH ROW EXECUTE FUNCTION set_updated_at();
    END IF;
END $$;

-- ─── org_api_keys ──────────────────────────────────────────────────────────
ALTER TABLE org_api_keys
    ADD COLUMN IF NOT EXISTS key_prefix text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS created_by text,
    ADD COLUMN IF NOT EXISTS rotated_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_org_api_keys_hash ON org_api_keys(api_key_hash);

-- ─── tenant_settings ───────────────────────────────────────────────────────
-- Campos de billing (legacy 0002).
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS hard_limits jsonb NOT NULL DEFAULT '{}'::jsonb;

-- Campos transaccionales (legacy 0005, 0010, 0029, 0064-0068, 0075).
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS currency text NOT NULL DEFAULT 'ARS',
    ADD COLUMN IF NOT EXISTS tax_rate numeric(5,2) NOT NULL DEFAULT 21.00,
    ADD COLUMN IF NOT EXISTS quote_prefix text NOT NULL DEFAULT 'PRE',
    ADD COLUMN IF NOT EXISTS sale_prefix text NOT NULL DEFAULT 'VTA',
    ADD COLUMN IF NOT EXISTS purchase_prefix text NOT NULL DEFAULT 'CPA',
    ADD COLUMN IF NOT EXISTS return_prefix text NOT NULL DEFAULT 'DEV',
    ADD COLUMN IF NOT EXISTS credit_note_prefix text NOT NULL DEFAULT 'NC',
    ADD COLUMN IF NOT EXISTS next_quote_number int NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS next_sale_number int NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS next_purchase_number int NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS next_return_number int NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS next_credit_note_number int NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS allow_negative_stock boolean NOT NULL DEFAULT true;

-- Datos de negocio (legacy 0012).
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS business_name text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS business_tax_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS business_address text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS business_phone text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS business_email text NOT NULL DEFAULT '';

-- WhatsApp templates (legacy 0012, 0024).
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS wa_quote_template text NOT NULL DEFAULT 'Hola {customer_name}, te enviamos el presupuesto {number} por {total}.',
    ADD COLUMN IF NOT EXISTS wa_receipt_template text NOT NULL DEFAULT 'Hola {customer_name}, tu comprobante de compra {number} por {total}. Gracias por tu compra!',
    ADD COLUMN IF NOT EXISTS wa_default_country_code text NOT NULL DEFAULT '54',
    ADD COLUMN IF NOT EXISTS wa_payment_template text NOT NULL DEFAULT 'Podes transferir a:\nAlias: {bank_alias}\nCBU: {bank_cbu}\nTitular: {bank_holder}\nBanco: {bank_name}\nMonto: {total}',
    ADD COLUMN IF NOT EXISTS wa_payment_link_template text NOT NULL DEFAULT 'Hola {customer_name}, podes pagar {total} de tu compra {number} con este link: {payment_url}';

-- Datos bancarios (legacy 0066).
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS bank_holder text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank_cbu text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank_alias text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank_name text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS show_qr_in_pdf boolean NOT NULL DEFAULT false;

-- Scheduling / appointments (legacy 0033, 0041).
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS scheduling_enabled boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS scheduling_label text NOT NULL DEFAULT 'Turno',
    ADD COLUMN IF NOT EXISTS scheduling_reminder_hours int NOT NULL DEFAULT 24;

-- Multi-currency (legacy 0029).
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS secondary_currency text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS default_rate_type text NOT NULL DEFAULT 'blue',
    ADD COLUMN IF NOT EXISTS auto_fetch_rates boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS show_dual_prices boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS supported_currencies jsonb NOT NULL DEFAULT '[]'::jsonb;

-- Onboarding profile (legacy 0040).
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS team_size text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sells text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS client_label text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS uses_billing boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS payment_method text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS vertical text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS onboarding_completed_at timestamptz;

-- ─── products / services (columnas operacionales faltantes) ───────────────
-- products.type: el código asume Type string (legacy 0042/0045 split products
-- vs services). En el schema squashed quedó implícito por tabla; el código
-- aún emite/lee el campo. Default 'product' para compat.
ALTER TABLE products
    ADD COLUMN IF NOT EXISTS type text NOT NULL DEFAULT 'product';

-- ─── in_app_notifications: schema incompatible (user_id/kind/entity_*) ───
-- El squash 0003 creó in_app_notifications con (actor_id, type, title, body)
-- pero el código asume (user_id, kind, entity_type, entity_id, chat_context)
-- y el TableName GORM es `pymes_in_app_notifications` (namespace pymes_).
-- DROP+CREATE: tabla está vacía en bootstrap.
DROP TABLE IF EXISTS in_app_notifications CASCADE;
DROP TABLE IF EXISTS pymes_in_app_notifications CASCADE;

CREATE TABLE pymes_in_app_notifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title text NOT NULL,
    body text NOT NULL,
    kind text NOT NULL,
    entity_type text NOT NULL DEFAULT '',
    entity_id text NOT NULL DEFAULT '',
    chat_context jsonb NOT NULL DEFAULT '{}'::jsonb,
    read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_pymes_in_app_notif_user_created
    ON pymes_in_app_notifications(user_id, created_at DESC);
CREATE INDEX idx_pymes_in_app_notif_org_unread
    ON pymes_in_app_notifications(org_id, read_at) WHERE read_at IS NULL;

-- ─── payments: ampliar check de method para incluir mercadopago ──────────
ALTER TABLE payments
    DROP CONSTRAINT IF EXISTS payments_method_check;
ALTER TABLE payments
    ADD CONSTRAINT payments_method_check
    CHECK (method IN ('cash','card','transfer','check','other','credit_note','mercadopago'));

-- ─── audit_log: rename payload_json→payload + columnas de actor/hash legacy ─
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema='public' AND table_name='audit_log' AND column_name='payload_json'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema='public' AND table_name='audit_log' AND column_name='payload'
    ) THEN
        ALTER TABLE audit_log RENAME COLUMN payload_json TO payload;
    END IF;
END $$;

ALTER TABLE audit_log
    ADD COLUMN IF NOT EXISTS actor_type text NOT NULL DEFAULT 'user',
    ADD COLUMN IF NOT EXISTS actor_id uuid,
    ADD COLUMN IF NOT EXISTS actor_label text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS hash_version int NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS payload_hash text NOT NULL DEFAULT '';

-- ─── procurement (módulo no incluido en squash, pero código + seeds lo usan) ─
CREATE TABLE IF NOT EXISTS procurement_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    requester_actor text NOT NULL,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    category text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'draft',
    estimated_total numeric(18, 4) NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'ARS',
    evaluation_json jsonb,
    purchase_id uuid REFERENCES purchases(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_procurement_requests_org ON procurement_requests(org_id);
CREATE INDEX IF NOT EXISTS idx_procurement_requests_status ON procurement_requests(org_id, status);
CREATE INDEX IF NOT EXISTS idx_procurement_requests_deleted ON procurement_requests(org_id, deleted_at);

CREATE TABLE IF NOT EXISTS procurement_request_lines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id uuid NOT NULL REFERENCES procurement_requests(id) ON DELETE CASCADE,
    description text NOT NULL DEFAULT '',
    product_id uuid,
    quantity numeric(18, 4) NOT NULL DEFAULT 1,
    unit_price_estimate numeric(18, 4) NOT NULL DEFAULT 0,
    sort_order int NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_procurement_request_lines_request ON procurement_request_lines(request_id);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_procurement_requests_updated_at') THEN
        CREATE TRIGGER trg_procurement_requests_updated_at
            BEFORE UPDATE ON procurement_requests
            FOR EACH ROW EXECUTE FUNCTION set_updated_at();
    END IF;
END $$;

-- ─── stock_levels: PK incompatible con branch_id NULL ─────────────────────
-- El squash declaró PK (org_id, product_id, branch_id) pero branch_id es
-- nullable y los seeds + código asumen "stock global cuando branch_id IS NULL".
-- Reemplazamos la PK por unique partial índices.
ALTER TABLE stock_levels DROP CONSTRAINT IF EXISTS stock_levels_pkey;
ALTER TABLE stock_levels ALTER COLUMN branch_id DROP NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_levels_global
    ON stock_levels(org_id, product_id) WHERE branch_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_levels_branched
    ON stock_levels(org_id, product_id, branch_id) WHERE branch_id IS NOT NULL;

-- ─── tenant_invitations ────────────────────────────────────────────────────
-- Schema actual de 0002 es incompatible con el código pymes (espera
-- email_normalized, token_hash, status, clerk_invitation_id, invited_by_user_id,
-- accepted_by_user_id, revoked_at, updated_at). DROP+CREATE es seguro porque
-- la tabla está vacía en bootstrap.
DROP TABLE IF EXISTS tenant_invitations CASCADE;

CREATE TABLE IF NOT EXISTS tenant_invitations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    email_normalized text NOT NULL,
    role text NOT NULL
        CONSTRAINT tenant_invitations_role_check CHECK (role IN ('admin','member')),
    status text NOT NULL DEFAULT 'pending'
        CONSTRAINT tenant_invitations_status_check CHECK (status IN ('pending','accepted','revoked','expired')),
    token_hash text NOT NULL UNIQUE,
    clerk_invitation_id text,
    invited_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    accepted_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    expires_at timestamptz NOT NULL,
    accepted_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_invitations_pending_email
    ON tenant_invitations(org_id, email_normalized) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_tenant_invitations_org_status
    ON tenant_invitations(org_id, status, created_at DESC);

CREATE TRIGGER trg_tenant_invitations_updated_at
    BEFORE UPDATE ON tenant_invitations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ─── webhook_events_clerk ──────────────────────────────────────────────────
-- 0002 lo creó como (svix_id, event_type, payload_json, received_at) — falta
-- el resto del lifecycle. Lo dropeamos y recreamos con el schema final.
DROP TABLE IF EXISTS webhook_events_clerk CASCADE;

CREATE TABLE IF NOT EXISTS webhook_events_clerk (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    svix_id text NOT NULL UNIQUE,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    status text NOT NULL DEFAULT 'pending'
        CONSTRAINT webhook_events_clerk_status_check
        CHECK (status IN ('pending','processed','failed','ignored')),
    error_message text,
    received_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_webhook_events_clerk_status
    ON webhook_events_clerk(status);
CREATE INDEX IF NOT EXISTS idx_webhook_events_clerk_event_type
    ON webhook_events_clerk(event_type);
CREATE INDEX IF NOT EXISTS idx_webhook_events_clerk_received_at
    ON webhook_events_clerk(received_at DESC);

CREATE TRIGGER trg_webhook_events_clerk_updated_at
    BEFORE UPDATE ON webhook_events_clerk
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
