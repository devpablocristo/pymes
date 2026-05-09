-- 0005_commercial.up.sql
-- Catálogo comercial: products, services, price_lists, price_list_items,
-- service_price_list_items.
--
-- Consolida: pymes-core/0005_core_business (productos), 0010_transversal_core
-- (price_lists), 0042_split_products_services (services), 0043_rename_service_tables,
-- 0049_products_services_is_active, 0052/0056_products_image_url(s),
-- 0066_internal_fields_commercial, 0067_price_lists_recurring_deleted_at.
--
-- Suppliers/customers ya no son tablas — se modelan como party_role.
-- Agrega FK party_roles.price_list_id que el 0004 dejó pendiente.

CREATE TABLE IF NOT EXISTS products (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    sku text,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    unit text NOT NULL DEFAULT 'unit',
    price numeric(15,2) NOT NULL DEFAULT 0,
    cost_price numeric(15,2) NOT NULL DEFAULT 0,
    price_currency text NOT NULL DEFAULT 'ARS',
    tax_rate numeric(5,2),
    track_stock boolean NOT NULL DEFAULT true,
    is_active boolean NOT NULL DEFAULT true,
    is_favorite boolean NOT NULL DEFAULT false,
    image_url text NOT NULL DEFAULT '',
    image_urls text[] NOT NULL DEFAULT '{}',
    tags text[] NOT NULL DEFAULT '{}',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_products_org_sku_uniq
    ON products(org_id, sku)
    WHERE archived_at IS NULL AND sku IS NOT NULL AND sku != '';
CREATE INDEX IF NOT EXISTS idx_products_org
    ON products(org_id) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_products_org_name
    ON products(org_id, name) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_products_org_archived_at
    ON products(org_id, archived_at);

CREATE TRIGGER trg_products_updated_at
    BEFORE UPDATE ON products FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS services (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    code text,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    category_code text NOT NULL DEFAULT '',
    sale_price numeric(15,2) NOT NULL DEFAULT 0,
    cost_price numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2),
    currency text NOT NULL DEFAULT 'ARS',
    default_duration_minutes integer,
    is_active boolean NOT NULL DEFAULT true,
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_services_org_code_uniq
    ON services(org_id, code)
    WHERE archived_at IS NULL AND code IS NOT NULL AND code != '';
CREATE INDEX IF NOT EXISTS idx_services_org
    ON services(org_id) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_services_org_name
    ON services(org_id, name) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_services_org_archived_at
    ON services(org_id, archived_at);

CREATE TRIGGER trg_services_updated_at
    BEFORE UPDATE ON services FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS price_lists (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    is_default boolean NOT NULL DEFAULT false,
    markup numeric(5,2) NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT true,
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    CONSTRAINT price_lists_org_name_uniq UNIQUE (org_id, name)
);
CREATE INDEX IF NOT EXISTS idx_price_lists_org
    ON price_lists(org_id) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_price_lists_org_archived_at
    ON price_lists(org_id, archived_at);

CREATE TRIGGER trg_price_lists_updated_at
    BEFORE UPDATE ON price_lists FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS price_list_items (
    price_list_id uuid NOT NULL REFERENCES price_lists(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    price numeric(15,2) NOT NULL,
    PRIMARY KEY (price_list_id, product_id)
);

CREATE TABLE IF NOT EXISTS service_price_list_items (
    price_list_id uuid NOT NULL REFERENCES price_lists(id) ON DELETE CASCADE,
    service_id uuid NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    price numeric(15,2) NOT NULL DEFAULT 0,
    PRIMARY KEY (price_list_id, service_id)
);

-- Cierra el FK pendiente de party_roles → price_lists (introducido acá porque
-- party_roles vive en 0004 y price_lists en 0005).
ALTER TABLE party_roles
    ADD COLUMN IF NOT EXISTS price_list_id uuid;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'party_roles_price_list_id_fkey'
    ) THEN
        ALTER TABLE party_roles
            ADD CONSTRAINT party_roles_price_list_id_fkey
            FOREIGN KEY (price_list_id)
            REFERENCES price_lists(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_party_roles_price_list
    ON party_roles(price_list_id) WHERE price_list_id IS NOT NULL;
