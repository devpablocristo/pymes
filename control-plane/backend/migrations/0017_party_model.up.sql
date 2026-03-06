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
SELECT id, id FROM customers;

CREATE TEMP TABLE supplier_party_map (
    supplier_id uuid PRIMARY KEY,
    party_id uuid NOT NULL
) ON COMMIT DROP;

INSERT INTO supplier_party_map (supplier_id, party_id)
SELECT
    s.id,
    COALESCE(
        (
            SELECT c.id
            FROM customers c
            WHERE c.org_id = s.org_id
              AND c.deleted_at IS NULL
              AND s.deleted_at IS NULL
              AND COALESCE(NULLIF(TRIM(c.tax_id), ''), '') <> ''
              AND c.tax_id = s.tax_id
            ORDER BY c.created_at ASC
            LIMIT 1
        ),
        (
            SELECT c.id
            FROM customers c
            WHERE c.org_id = s.org_id
              AND c.deleted_at IS NULL
              AND s.deleted_at IS NULL
              AND LOWER(TRIM(c.name)) = LOWER(TRIM(s.name))
              AND COALESCE(NULLIF(LOWER(TRIM(c.email)), ''), '') = COALESCE(NULLIF(LOWER(TRIM(s.email)), ''), '')
            ORDER BY c.created_at ASC
            LIMIT 1
        ),
        s.id
    ) AS party_id
FROM suppliers s;

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
    NULLIF(TRIM(regexp_replace(TRIM(c.name), '^[^ ]+\\s*', '')), '')
FROM customers c
WHERE c.type = 'person'
ON CONFLICT (party_id) DO NOTHING;

INSERT INTO party_organizations (party_id, legal_name, trade_name, tax_condition)
SELECT
    c.id,
    c.name,
    c.name,
    COALESCE(c.metadata->>'tax_condition', '')
FROM customers c
WHERE c.type = 'company'
ON CONFLICT (party_id) DO NOTHING;

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
    COALESCE(s.metadata, '{}'::jsonb) || CASE WHEN COALESCE(NULLIF(TRIM(s.contact_name), ''), '') = '' THEN '{}'::jsonb ELSE jsonb_build_object('contact_name', s.contact_name) END,
    s.created_at,
    s.updated_at,
    s.deleted_at
FROM suppliers s
JOIN supplier_party_map spm ON spm.supplier_id = s.id
WHERE spm.party_id = s.id
ON CONFLICT (id) DO NOTHING;

INSERT INTO party_organizations (party_id, legal_name, trade_name, tax_condition)
SELECT
    spm.party_id,
    s.name,
    s.name,
    COALESCE(s.metadata->>'tax_condition', '')
FROM suppliers s
JOIN supplier_party_map spm ON spm.supplier_id = s.id
WHERE spm.party_id = s.id
ON CONFLICT (party_id) DO NOTHING;

UPDATE parties p
SET
    email = COALESCE(p.email, NULLIF(s.email, '')),
    phone = COALESCE(p.phone, NULLIF(s.phone, '')),
    tax_id = COALESCE(p.tax_id, NULLIF(s.tax_id, '')),
    address = CASE WHEN p.address = '{}'::jsonb THEN COALESCE(s.address, '{}'::jsonb) ELSE p.address END,
    metadata = COALESCE(p.metadata, '{}'::jsonb)
        || COALESCE(s.metadata, '{}'::jsonb)
        || CASE WHEN COALESCE(NULLIF(TRIM(s.contact_name), ''), '') = '' THEN '{}'::jsonb ELSE jsonb_build_object('contact_name', s.contact_name) END,
    updated_at = GREATEST(p.updated_at, s.updated_at)
FROM suppliers s
JOIN supplier_party_map spm ON spm.supplier_id = s.id
WHERE p.id = spm.party_id;

INSERT INTO party_roles (party_id, org_id, role, is_active, price_list_id, metadata, created_at)
SELECT c.id, c.org_id, 'customer', true, c.price_list_id, '{}'::jsonb, c.created_at
FROM customers c
ON CONFLICT (party_id, org_id, role) DO NOTHING;

INSERT INTO party_roles (party_id, org_id, role, is_active, price_list_id, metadata, created_at)
SELECT DISTINCT
    spm.party_id,
    s.org_id,
    'supplier',
    true,
    NULL,
    CASE WHEN COALESCE(NULLIF(TRIM(s.contact_name), ''), '') = '' THEN '{}'::jsonb ELSE jsonb_build_object('contact_name', s.contact_name) END,
    s.created_at
FROM suppliers s
JOIN supplier_party_map spm ON spm.supplier_id = s.id
ON CONFLICT (party_id, org_id, role) DO NOTHING;

WITH members AS (
    SELECT
        om.id AS org_member_id,
        om.org_id,
        u.id AS user_id,
        u.external_id,
        COALESCE(NULLIF(TRIM(u.name), ''), u.email) AS display_name,
        u.email,
        gen_random_uuid() AS generated_party_id
    FROM org_members om
    JOIN users u ON u.id = om.user_id
    WHERE om.party_id IS NULL
)
INSERT INTO parties (
    id, org_id, party_type, display_name, email, metadata, created_at, updated_at
)
SELECT
    generated_party_id,
    org_id,
    'person',
    display_name,
    email,
    jsonb_build_object('system_key', 'org_member', 'user_external_id', external_id),
    now(),
    now()
FROM members
ON CONFLICT (id) DO NOTHING;

WITH members AS (
    SELECT
        om.id AS org_member_id,
        om.org_id,
        om.party_id,
        u.external_id,
        COALESCE(NULLIF(TRIM(u.name), ''), u.email) AS display_name
    FROM org_members om
    JOIN users u ON u.id = om.user_id
    WHERE om.party_id IS NOT NULL
)
INSERT INTO party_persons (party_id, first_name, last_name)
SELECT
    party_id,
    split_part(TRIM(display_name), ' ', 1),
    NULLIF(TRIM(regexp_replace(TRIM(display_name), '^[^ ]+\\s*', '')), '')
FROM members
ON CONFLICT (party_id) DO NOTHING;

WITH members AS (
    SELECT
        om.id AS org_member_id,
        om.org_id,
        u.id AS user_id,
        gen_random_uuid() AS generated_party_id
    FROM org_members om
    JOIN users u ON u.id = om.user_id
    WHERE om.party_id IS NULL
), updated AS (
    UPDATE org_members om
    SET party_id = m.generated_party_id
    FROM members m
    WHERE om.id = m.org_member_id
    RETURNING om.org_id, om.party_id
)
INSERT INTO party_roles (party_id, org_id, role, is_active, price_list_id, metadata, created_at)
SELECT party_id, org_id, 'employee', true, NULL, '{}'::jsonb, now()
FROM updated
ON CONFLICT (party_id, org_id, role) DO NOTHING;

INSERT INTO party_roles (party_id, org_id, role, is_active, price_list_id, metadata, created_at)
SELECT om.party_id, om.org_id, 'employee', true, NULL, '{}'::jsonb, now()
FROM org_members om
WHERE om.party_id IS NOT NULL
ON CONFLICT (party_id, org_id, role) DO NOTHING;

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
    SELECT 1 FROM parties p
    WHERE p.org_id = o.id
      AND p.party_type = 'automated_agent'
      AND p.metadata->>'system_key' = 'assistant_ai'
)
ON CONFLICT DO NOTHING;

INSERT INTO party_agents (party_id, agent_kind, provider, config, is_active)
SELECT p.id, 'ai', 'gemini', jsonb_build_object('model', 'gemini-2.0-flash'), true
FROM parties p
WHERE p.party_type = 'automated_agent'
  AND p.metadata->>'system_key' = 'assistant_ai'
ON CONFLICT (party_id) DO NOTHING;

INSERT INTO party_roles (party_id, org_id, role, is_active, price_list_id, metadata, created_at)
SELECT p.id, p.org_id, 'assistant', true, NULL, '{}'::jsonb, p.created_at
FROM parties p
WHERE p.party_type = 'automated_agent'
  AND p.metadata->>'system_key' = 'assistant_ai'
ON CONFLICT (party_id, org_id, role) DO NOTHING;

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
        WHERE table_name = 'quotes' AND column_name = 'customer_id'
    ) THEN
        ALTER TABLE quotes RENAME COLUMN customer_id TO party_id;
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'quotes' AND column_name = 'customer_name'
    ) THEN
        ALTER TABLE quotes RENAME COLUMN customer_name TO party_name;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sales' AND column_name = 'customer_id'
    ) THEN
        ALTER TABLE sales RENAME COLUMN customer_id TO party_id;
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sales' AND column_name = 'customer_name'
    ) THEN
        ALTER TABLE sales RENAME COLUMN customer_name TO party_name;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'purchases' AND column_name = 'supplier_id'
    ) THEN
        ALTER TABLE purchases RENAME COLUMN supplier_id TO party_id;
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'purchases' AND column_name = 'supplier_name'
    ) THEN
        ALTER TABLE purchases RENAME COLUMN supplier_name TO party_name;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'recurring_expenses' AND column_name = 'supplier_id'
    ) THEN
        ALTER TABLE recurring_expenses RENAME COLUMN supplier_id TO party_id;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'appointments' AND column_name = 'customer_id'
    ) THEN
        ALTER TABLE appointments RENAME COLUMN customer_id TO party_id;
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'appointments' AND column_name = 'customer_name'
    ) THEN
        ALTER TABLE appointments RENAME COLUMN customer_name TO party_name;
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'appointments' AND column_name = 'customer_phone'
    ) THEN
        ALTER TABLE appointments RENAME COLUMN customer_phone TO party_phone;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'credit_notes' AND column_name = 'customer_id'
    ) THEN
        ALTER TABLE credit_notes RENAME COLUMN customer_id TO party_id;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'accounts' AND column_name = 'entity_id'
    ) THEN
        ALTER TABLE accounts RENAME COLUMN entity_id TO party_id;
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'accounts' AND column_name = 'entity_name'
    ) THEN
        ALTER TABLE accounts RENAME COLUMN entity_name TO party_name;
    END IF;
END $$;

UPDATE purchases p
SET party_id = spm.party_id
FROM supplier_party_map spm
WHERE p.party_id = spm.supplier_id;

UPDATE recurring_expenses re
SET party_id = spm.party_id
FROM supplier_party_map spm
WHERE re.party_id = spm.supplier_id;

UPDATE accounts a
SET party_id = CASE
    WHEN a.entity_type = 'customer' THEN cpm.party_id
    WHEN a.entity_type = 'supplier' THEN spm.party_id
    ELSE a.party_id
END
FROM customer_party_map cpm
FULL JOIN supplier_party_map spm ON false
WHERE (a.entity_type = 'customer' AND a.party_id = cpm.customer_id)
   OR (a.entity_type = 'supplier' AND a.party_id = spm.supplier_id);

ALTER TABLE accounts DROP COLUMN IF EXISTS entity_type;

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

ALTER TABLE quotes DROP CONSTRAINT IF EXISTS quotes_customer_id_fkey;
ALTER TABLE sales DROP CONSTRAINT IF EXISTS sales_customer_id_fkey;
ALTER TABLE purchases DROP CONSTRAINT IF EXISTS purchases_supplier_id_fkey;
ALTER TABLE recurring_expenses DROP CONSTRAINT IF EXISTS recurring_expenses_supplier_id_fkey;
ALTER TABLE appointments DROP CONSTRAINT IF EXISTS appointments_customer_id_fkey;
ALTER TABLE credit_notes DROP CONSTRAINT IF EXISTS credit_notes_customer_id_fkey;
ALTER TABLE accounts DROP CONSTRAINT IF EXISTS accounts_party_id_fkey;
ALTER TABLE payment_preferences DROP CONSTRAINT IF EXISTS payment_preferences_party_id_fkey;
ALTER TABLE ai_conversations DROP CONSTRAINT IF EXISTS ai_conversations_agent_party_id_fkey;
ALTER TABLE org_members DROP CONSTRAINT IF EXISTS org_members_party_id_fkey;
ALTER TABLE user_roles DROP CONSTRAINT IF EXISTS user_roles_party_id_fkey;

ALTER TABLE quotes ADD CONSTRAINT quotes_party_id_fkey FOREIGN KEY (party_id) REFERENCES parties(id);
ALTER TABLE sales ADD CONSTRAINT sales_party_id_fkey FOREIGN KEY (party_id) REFERENCES parties(id);
ALTER TABLE purchases ADD CONSTRAINT purchases_party_id_fkey FOREIGN KEY (party_id) REFERENCES parties(id);
ALTER TABLE recurring_expenses ADD CONSTRAINT recurring_expenses_party_id_fkey FOREIGN KEY (party_id) REFERENCES parties(id);
ALTER TABLE appointments ADD CONSTRAINT appointments_party_id_fkey FOREIGN KEY (party_id) REFERENCES parties(id);
ALTER TABLE credit_notes ADD CONSTRAINT credit_notes_party_id_fkey FOREIGN KEY (party_id) REFERENCES parties(id);
ALTER TABLE accounts ADD CONSTRAINT accounts_party_id_fkey FOREIGN KEY (party_id) REFERENCES parties(id) ON DELETE CASCADE;
ALTER TABLE payment_preferences ADD CONSTRAINT payment_preferences_party_id_fkey FOREIGN KEY (party_id) REFERENCES parties(id);
ALTER TABLE ai_conversations ADD CONSTRAINT ai_conversations_agent_party_id_fkey FOREIGN KEY (agent_party_id) REFERENCES parties(id);
ALTER TABLE org_members ADD CONSTRAINT org_members_party_id_fkey FOREIGN KEY (party_id) REFERENCES parties(id);
ALTER TABLE user_roles ADD CONSTRAINT user_roles_party_id_fkey FOREIGN KEY (party_id) REFERENCES parties(id);

DROP INDEX IF EXISTS idx_quotes_customer;
DROP INDEX IF EXISTS idx_sales_customer;
DROP INDEX IF EXISTS idx_purchases_supplier;
DROP INDEX IF EXISTS idx_credit_notes_customer;
DROP INDEX IF EXISTS idx_appointments_customer;
DROP INDEX IF EXISTS idx_accounts_entity;
DROP INDEX IF EXISTS idx_customers_org_tax_unique;
DROP INDEX IF EXISTS idx_customers_org;
DROP INDEX IF EXISTS idx_customers_org_name;
DROP INDEX IF EXISTS idx_customers_org_email;
DROP INDEX IF EXISTS idx_customers_org_tax;
DROP INDEX IF EXISTS idx_customers_tags;
DROP INDEX IF EXISTS idx_suppliers_org_tax_unique;
DROP INDEX IF EXISTS idx_suppliers_org;
DROP INDEX IF EXISTS idx_suppliers_org_name;
DROP INDEX IF EXISTS idx_suppliers_org_tax;

CREATE INDEX IF NOT EXISTS idx_quotes_party ON quotes(party_id) WHERE party_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_sales_party ON sales(party_id) WHERE party_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_purchases_party ON purchases(party_id) WHERE party_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_credit_notes_party ON credit_notes(org_id, party_id) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_appointments_party ON appointments(party_id) WHERE party_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_accounts_party ON accounts(org_id, party_id);
CREATE INDEX IF NOT EXISTS idx_payment_preferences_party ON payment_preferences(org_id, party_id) WHERE party_id IS NOT NULL;

DROP TABLE IF EXISTS customers;
DROP TABLE IF EXISTS suppliers;

CREATE OR REPLACE VIEW customers AS
SELECT
    p.id,
    p.org_id,
    CASE WHEN p.party_type = 'organization' THEN 'company' ELSE 'person' END AS type,
    p.display_name AS name,
    p.tax_id,
    p.email,
    p.phone,
    p.address,
    p.notes,
    p.tags,
    p.metadata,
    pr.price_list_id,
    p.created_at,
    p.updated_at,
    p.deleted_at
FROM parties p
JOIN party_roles r
    ON r.party_id = p.id
   AND r.org_id = p.org_id
   AND r.role = 'customer'
   AND r.is_active = true
LEFT JOIN party_roles pr
    ON pr.party_id = p.id
   AND pr.org_id = p.org_id
   AND pr.role = 'customer';

CREATE OR REPLACE VIEW suppliers AS
SELECT
    p.id,
    p.org_id,
    p.display_name AS name,
    p.tax_id,
    p.email,
    p.phone,
    p.address,
    COALESCE(r.metadata->>'contact_name', p.metadata->>'contact_name', '') AS contact_name,
    p.notes,
    p.tags,
    p.metadata,
    p.created_at,
    p.updated_at,
    p.deleted_at
FROM parties p
JOIN party_roles r
    ON r.party_id = p.id
   AND r.org_id = p.org_id
   AND r.role = 'supplier'
   AND r.is_active = true;
