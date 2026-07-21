-- 0004_party_model.up.sql
-- Modelo unificado de "parties" (personas / organizaciones / agentes automáticos).
-- Reemplaza customers/suppliers de core/0005 (legacy) — esos roles ahora
-- son `party_roles` con `role IN ('customer','supplier','contact', ...)`.
--
-- Nota: party_roles.price_list_id se introduce en 0005 (cuando price_lists
-- ya existe) vía ALTER TABLE. Acá no se declara la FK.

CREATE TABLE IF NOT EXISTS parties (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    party_type text NOT NULL
        CONSTRAINT parties_party_type_check
        CHECK (party_type IN ('person','organization','automated_agent')),
    display_name text NOT NULL,
    email text,
    phone text,
    address jsonb NOT NULL DEFAULT '{}'::jsonb,
    tax_id text,
    notes text NOT NULL DEFAULT '',
    tags text[] NOT NULL DEFAULT '{}',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    is_favorite boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_parties_org_tax_uniq
    ON parties(org_id, tax_id)
    WHERE deleted_at IS NULL AND tax_id IS NOT NULL AND tax_id != '';
CREATE INDEX IF NOT EXISTS idx_parties_org
    ON parties(org_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_parties_org_type
    ON parties(org_id, party_type) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_parties_org_name
    ON parties(org_id, display_name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_parties_org_email
    ON parties(org_id, email) WHERE deleted_at IS NULL AND email IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_parties_tags
    ON parties USING GIN(tags) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_parties_updated_at
    BEFORE UPDATE ON parties FOR EACH ROW EXECUTE FUNCTION set_updated_at();

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
    agent_kind text NOT NULL
        CONSTRAINT party_agents_agent_kind_check
        CHECK (agent_kind IN ('ai','service','integration','bot')),
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
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT party_roles_party_org_role_uniq UNIQUE (party_id, org_id, role)
);
CREATE INDEX IF NOT EXISTS idx_party_roles_org_role
    ON party_roles(org_id, role) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_party_roles_party
    ON party_roles(party_id);

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

CREATE TABLE IF NOT EXISTS party_classifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    party_id uuid NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    classification text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT party_classifications_party_classification_uniq
        UNIQUE (party_id, classification)
);
CREATE INDEX IF NOT EXISTS idx_party_classifications_org
    ON party_classifications(org_id);

CREATE TABLE IF NOT EXISTS party_contacts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    party_id uuid NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    contact_type text NOT NULL
        CONSTRAINT party_contacts_contact_type_check
        CHECK (contact_type IN ('phone','email','social','other')),
    contact_value text NOT NULL,
    is_primary boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT party_contacts_party_type_value_uniq
        UNIQUE (party_id, contact_type, contact_value)
);
CREATE INDEX IF NOT EXISTS idx_party_contacts_org ON party_contacts(org_id);
CREATE INDEX IF NOT EXISTS idx_party_contacts_party ON party_contacts(party_id);

CREATE TRIGGER trg_party_contacts_updated_at
    BEFORE UPDATE ON party_contacts FOR EACH ROW EXECUTE FUNCTION set_updated_at();
