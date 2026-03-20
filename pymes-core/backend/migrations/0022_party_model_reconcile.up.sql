CREATE TABLE IF NOT EXISTS parties (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    party_type text NOT NULL CHECK (party_type IN ('person', 'organization', 'automated_agent')),
    display_name text NOT NULL,
    email text,
    phone text,
    address jsonb NOT NULL DEFAULT '{}'::jsonb,
    tax_id text,
    notes text NOT NULL DEFAULT '',
    tags text[] NOT NULL DEFAULT '{}',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_parties_org ON parties(org_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_parties_org_type ON parties(org_id, party_type) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_parties_org_name ON parties(org_id, display_name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_parties_org_email ON parties(org_id, email) WHERE deleted_at IS NULL AND email IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_parties_org_tax ON parties(org_id, tax_id) WHERE deleted_at IS NULL AND tax_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_parties_tags ON parties USING GIN(tags) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS party_persons (
    party_id uuid PRIMARY KEY REFERENCES parties(id) ON DELETE CASCADE,
    first_name text NOT NULL DEFAULT '',
    last_name text NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS party_organizations (
    party_id uuid PRIMARY KEY REFERENCES parties(id) ON DELETE CASCADE,
    legal_name text NOT NULL DEFAULT '',
    trade_name text NOT NULL DEFAULT '',
    tax_condition text NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS party_agents (
    party_id uuid PRIMARY KEY REFERENCES parties(id) ON DELETE CASCADE,
    agent_kind text NOT NULL CHECK (agent_kind IN ('ai', 'service', 'integration', 'bot')),
    provider text NOT NULL DEFAULT '',
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    is_active boolean NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS party_roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    party_id uuid NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    role text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    price_list_id uuid REFERENCES price_lists(id),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(party_id, org_id, role)
);

CREATE INDEX IF NOT EXISTS idx_party_roles_org_role ON party_roles(org_id, role) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_party_roles_party ON party_roles(party_id);

CREATE TABLE IF NOT EXISTS party_relationships (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    from_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
    to_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
    relationship_type text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    from_date timestamptz NOT NULL DEFAULT now(),
    thru_date timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_party_rels_org ON party_relationships(org_id);
CREATE INDEX IF NOT EXISTS idx_party_rels_from ON party_relationships(from_party_id);
CREATE INDEX IF NOT EXISTS idx_party_rels_to ON party_relationships(to_party_id);

CREATE TABLE IF NOT EXISTS services (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    direction text NOT NULL CHECK (direction IN ('inbound', 'outbound', 'internal')),
    kind text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE org_members ADD COLUMN IF NOT EXISTS party_id uuid;

ALTER TABLE audit_log
    ADD COLUMN IF NOT EXISTS actor_type text NOT NULL DEFAULT 'user' CHECK (actor_type IN ('user', 'party', 'service', 'system')),
    ADD COLUMN IF NOT EXISTS actor_id uuid,
    ADD COLUMN IF NOT EXISTS actor_label text NOT NULL DEFAULT '';

UPDATE audit_log
SET actor_label = COALESCE(NULLIF(actor_label, ''), COALESCE(actor, ''))
WHERE actor_label = '' OR actor_label IS NULL;

CREATE INDEX IF NOT EXISTS idx_audit_log_actor ON audit_log(org_id, actor_type, actor_id);

CREATE TEMP TABLE customer_party_map (
    customer_id uuid PRIMARY KEY,
    party_id uuid NOT NULL
) ON COMMIT DROP;

INSERT INTO customer_party_map (customer_id, party_id)
SELECT id, id FROM customers
ON CONFLICT (customer_id) DO NOTHING;

CREATE TEMP TABLE supplier_party_map (
    supplier_id uuid PRIMARY KEY,
    party_id uuid NOT NULL
) ON COMMIT DROP;

INSERT INTO supplier_party_map (supplier_id, party_id)
SELECT id, id FROM suppliers
ON CONFLICT (supplier_id) DO NOTHING;

INSERT INTO parties (
    id, org_id, party_type, display_name, email, phone, address, tax_id, notes, tags, metadata,
    created_at, updated_at, deleted_at
)
SELECT
    c.id,
    c.org_id,
    CASE WHEN c.type = 'company' THEN 'organization' ELSE 'person' END,
    c.name,
    NULLIF(c.email, ''),
    NULLIF(c.phone, ''),
    COALESCE(c.address, '{}'::jsonb),
    NULLIF(c.tax_id, ''),
    COALESCE(c.notes, ''),
    COALESCE(c.tags, '{}'::text[]),
    COALESCE(c.metadata, '{}'::jsonb),
    c.created_at,
    c.updated_at,
    c.deleted_at
FROM customers c
ON CONFLICT (id) DO NOTHING;

INSERT INTO party_persons (party_id, first_name, last_name)
SELECT
    c.id,
    split_part(TRIM(c.name), ' ', 1),
    NULLIF(TRIM(regexp_replace(TRIM(c.name), '^[^ ]+\s*', '')), '')
FROM customers c
WHERE c.type = 'person'
ON CONFLICT (party_id) DO UPDATE SET
    first_name = EXCLUDED.first_name,
    last_name = EXCLUDED.last_name;

INSERT INTO party_organizations (party_id, legal_name, trade_name, tax_condition)
SELECT
    c.id,
    c.name,
    c.name,
    COALESCE(c.metadata->>'tax_condition', '')
FROM customers c
WHERE c.type = 'company'
ON CONFLICT (party_id) DO UPDATE SET
    legal_name = EXCLUDED.legal_name,
    trade_name = EXCLUDED.trade_name,
    tax_condition = EXCLUDED.tax_condition;

INSERT INTO parties (
    id, org_id, party_type, display_name, email, phone, address, tax_id, notes, tags, metadata,
    created_at, updated_at, deleted_at
)
SELECT
    spm.party_id,
    s.org_id,
    'organization',
    s.name,
    NULLIF(s.email, ''),
    NULLIF(s.phone, ''),
    COALESCE(s.address, '{}'::jsonb),
    NULLIF(s.tax_id, ''),
    COALESCE(s.notes, ''),
    COALESCE(s.tags, '{}'::text[]),
    COALESCE(s.metadata, '{}'::jsonb)
        || CASE
            WHEN COALESCE(NULLIF(TRIM(s.contact_name), ''), '') = '' THEN '{}'::jsonb
            ELSE jsonb_build_object('contact_name', s.contact_name)
        END,
    s.created_at,
    s.updated_at,
    s.deleted_at
FROM suppliers s
JOIN supplier_party_map spm ON spm.supplier_id = s.id
ON CONFLICT (id) DO NOTHING;

INSERT INTO party_organizations (party_id, legal_name, trade_name, tax_condition)
SELECT
    spm.party_id,
    s.name,
    s.name,
    COALESCE(s.metadata->>'tax_condition', '')
FROM suppliers s
JOIN supplier_party_map spm ON spm.supplier_id = s.id
ON CONFLICT (party_id) DO UPDATE SET
    legal_name = EXCLUDED.legal_name,
    trade_name = EXCLUDED.trade_name,
    tax_condition = EXCLUDED.tax_condition;

INSERT INTO party_roles (id, party_id, org_id, role, is_active, price_list_id, metadata, created_at)
SELECT gen_random_uuid(), c.id, c.org_id, 'customer', true, c.price_list_id, '{}'::jsonb, c.created_at
FROM customers c
ON CONFLICT (party_id, org_id, role) DO UPDATE SET
    is_active = true,
    price_list_id = EXCLUDED.price_list_id;

INSERT INTO party_roles (id, party_id, org_id, role, is_active, price_list_id, metadata, created_at)
SELECT
    gen_random_uuid(),
    spm.party_id,
    s.org_id,
    'supplier',
    true,
    NULL::uuid,
    CASE
        WHEN COALESCE(NULLIF(TRIM(s.contact_name), ''), '') = '' THEN '{}'::jsonb
        ELSE jsonb_build_object('contact_name', s.contact_name)
    END,
    s.created_at
FROM suppliers s
JOIN supplier_party_map spm ON spm.supplier_id = s.id
ON CONFLICT (party_id, org_id, role) DO UPDATE SET
    is_active = true,
    metadata = EXCLUDED.metadata;

WITH members AS (
    SELECT
        om.id AS org_member_id,
        om.org_id,
        u.external_id,
        COALESCE(NULLIF(TRIM(u.name), ''), u.email) AS display_name,
        u.email
    FROM org_members om
    JOIN users u ON u.id = om.user_id
)
INSERT INTO parties (
    id, org_id, party_type, display_name, email, metadata, created_at, updated_at
)
SELECT
    gen_random_uuid(),
    org_id,
    'person',
    display_name,
    email,
    jsonb_build_object('system_key', 'org_member', 'user_external_id', external_id),
    now(),
    now()
FROM members
WHERE NOT EXISTS (
    SELECT 1
    FROM parties p
    WHERE p.org_id = members.org_id
      AND p.metadata->>'system_key' = 'org_member'
      AND p.metadata->>'user_external_id' = members.external_id
);

WITH members AS (
    SELECT
        om.id AS org_member_id,
        om.org_id,
        p.id AS party_id
    FROM org_members om
    JOIN users u ON u.id = om.user_id
    JOIN parties p
      ON p.org_id = om.org_id
     AND p.metadata->>'system_key' = 'org_member'
     AND p.metadata->>'user_external_id' = u.external_id
    WHERE om.party_id IS NULL
)
UPDATE org_members om
SET party_id = members.party_id
FROM members
WHERE om.id = members.org_member_id;

INSERT INTO party_persons (party_id, first_name, last_name)
SELECT
    om.party_id,
    split_part(TRIM(COALESCE(NULLIF(u.name, ''), u.email)), ' ', 1),
    NULLIF(TRIM(regexp_replace(TRIM(COALESCE(NULLIF(u.name, ''), u.email)), '^[^ ]+\s*', '')), '')
FROM org_members om
JOIN users u ON u.id = om.user_id
WHERE om.party_id IS NOT NULL
ON CONFLICT (party_id) DO NOTHING;

INSERT INTO party_roles (id, party_id, org_id, role, is_active, price_list_id, metadata, created_at)
SELECT gen_random_uuid(), om.party_id, om.org_id, 'employee', true, NULL::uuid, '{}'::jsonb, now()
FROM org_members om
WHERE om.party_id IS NOT NULL
ON CONFLICT (party_id, org_id, role) DO UPDATE SET is_active = true;

INSERT INTO parties (
    id, org_id, party_type, display_name, metadata, created_at, updated_at
)
SELECT
    gen_random_uuid(),
    o.id,
    'automated_agent',
    'Asistente AI',
    jsonb_build_object('system_key', 'assistant_ai'),
    now(),
    now()
FROM orgs o
WHERE NOT EXISTS (
    SELECT 1
    FROM parties p
    WHERE p.org_id = o.id
      AND p.party_type = 'automated_agent'
      AND p.metadata->>'system_key' = 'assistant_ai'
);

INSERT INTO party_agents (party_id, agent_kind, provider, config, is_active)
SELECT p.id, 'ai', 'gemini', jsonb_build_object('model', 'gemini-2.0-flash'), true
FROM parties p
WHERE p.party_type = 'automated_agent'
  AND p.metadata->>'system_key' = 'assistant_ai'
ON CONFLICT (party_id) DO NOTHING;

INSERT INTO party_roles (id, party_id, org_id, role, is_active, price_list_id, metadata, created_at)
SELECT gen_random_uuid(), p.id, p.org_id, 'assistant', true, NULL::uuid, '{}'::jsonb, p.created_at
FROM parties p
WHERE p.party_type = 'automated_agent'
  AND p.metadata->>'system_key' = 'assistant_ai'
ON CONFLICT (party_id, org_id, role) DO UPDATE SET is_active = true;

ALTER TABLE user_roles ADD COLUMN IF NOT EXISTS party_id uuid;

UPDATE user_roles ur
SET party_id = om.party_id
FROM org_members om
WHERE ur.org_id = om.org_id
  AND ur.user_id = om.user_id
  AND ur.party_id IS NULL;

ALTER TABLE ai_conversations ADD COLUMN IF NOT EXISTS agent_party_id uuid;

UPDATE ai_conversations ac
SET agent_party_id = p.id
FROM parties p
WHERE p.org_id = ac.org_id
  AND p.party_type = 'automated_agent'
  AND p.metadata->>'system_key' = 'assistant_ai'
  AND ac.agent_party_id IS NULL;

ALTER TABLE payment_preferences ADD COLUMN IF NOT EXISTS party_id uuid;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sales' AND column_name = 'party_id'
    ) THEN
        UPDATE payment_preferences pp
        SET party_id = s.party_id
        FROM sales s
        WHERE pp.reference_type = 'sale'
          AND pp.reference_id = s.id
          AND pp.party_id IS NULL;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'quotes' AND column_name = 'party_id'
    ) THEN
        UPDATE payment_preferences pp
        SET party_id = q.party_id
        FROM quotes q
        WHERE pp.reference_type = 'quote'
          AND pp.reference_id = q.id
          AND pp.party_id IS NULL;
    END IF;
END $$;

INSERT INTO services (name, direction, kind, description)
VALUES
    ('clerk_webhook', 'inbound', 'webhook', 'Clerk user/org sync'),
    ('stripe_webhook', 'inbound', 'webhook', 'Stripe billing events'),
    ('mercadopago_webhook', 'inbound', 'gateway', 'Mercado Pago IPN webhook'),
    ('mercadopago_api', 'outbound', 'gateway', 'Mercado Pago API'),
    ('scheduler', 'internal', 'scheduler', 'Periodic task runner'),
    ('email_notifications', 'outbound', 'notification', 'SES/SMTP email sender'),
    ('outgoing_webhooks', 'outbound', 'webhook', 'Webhook dispatcher to external URLs'),
    ('pdf_generator', 'internal', 'document', 'PDF renderer')
ON CONFLICT (name) DO NOTHING;
